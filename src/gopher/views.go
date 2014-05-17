/*
一些辅助方法
*/

package gopher

import (
	"crypto/md5"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/jimmykuu/webhelpers"
	"github.com/jimmykuu/wtforms"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
)

const (
	PerPage = 20
)

var (
	DB          *mgo.Database
	store       *sessions.CookieStore
	fileVersion map[string]string = make(map[string]string) // {path: version}
	utils       *Utils
)

type Utils struct {
}

// 没有http://开头的增加http://
func (u *Utils) Url(url string) string {
	if strings.HasPrefix(url, "http://") {
		return url
	}

	return "http://" + url
}

/*
for 循环作用域找不到这个工具
*/
func (u *Utils) StaticUrl(path string) string {

	version, ok := fileVersion[path]
	if ok {
		return "/static/" + path + "?v=" + version
	}

	file, err := os.Open("static/" + path)

	if err != nil {
		return "/static/" + path
	}

	h := md5.New()

	_, err = io.Copy(h, file)

	version = fmt.Sprintf("%x", h.Sum(nil))[:5]

	fileVersion[path] = version

	return "/static/" + path + "?v=" + version
}

func (u *Utils) Index(index int) int {
	return index + 1
}
func (u *Utils) FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}
func (u *Utils) FormatTime(t time.Time) string {
	now := time.Now()
	duration := now.Sub(t)
	if duration.Seconds() < 60 {
		return fmt.Sprintf("刚刚")
	} else if duration.Minutes() < 60 {
		return fmt.Sprintf("%.0f 分钟前", duration.Minutes())
	} else if duration.Hours() < 24 {
		return fmt.Sprintf("%.0f 小时前", duration.Hours())
	}

	t = t.Add(time.Hour * time.Duration(Config.TimeZoneOffset))
	return t.Format("2006-01-02 15:04")
}

func (u *Utils) UserInfo(username string) template.HTML {
	c := DB.C(USERS)

	user := User{}
	// 检查用户名
	c.Find(bson.M{"username": username}).One(&user)

	format := `<div>
        <a href="/member/%s"><img class="gravatar img-rounded" src="%s-middle" style="float:left;"></a>
        <h3><a href="/member/%s">%s</a><br><small>%s</small></h3>
        <div class="clearfix"></div>
    </div>`

	return template.HTML(fmt.Sprintf(format, username, user.AvatarImgSrc(), username, username, user.Tagline))
}

/*mark ggaaooppeenngg*/
func (u *Utils) RecentReplies(username string) template.HTML {
	c := DB.C(USERS)
	ccontens := DB.C(CONTENTS)
	user := User{}
	// 检查用户名
	c.Find(bson.M{"username": username}).One(&user)
	var anchors []string
	anchor := `<a href="/t/%s"  class="btn">%s</a><br>`
	for _, v := range user.RecentReplies {
		var topic Topic
		err := ccontens.Find(bson.M{"_id": bson.ObjectIdHex(v)}).One(&topic)
		if err != nil {
			fmt.Println(err)
		}
		anchors = append(anchors, fmt.Sprintf(anchor, topic.Id_.Hex(), topic.Title))
	}
	s := strings.Join(anchors, "\n")
	//最近被at
	var ats []string
	for _, v := range user.RecentAts {
		var topic Topic
		if err := ccontens.Find(bson.M{"_id": bson.ObjectIdHex(v)}).One(&topic); err != nil {
			fmt.Println(err)
		}
		ats = append(ats, fmt.Sprintf(anchor, topic.Id_.Hex(), topic.Title))
	}
	a := strings.Join(ats, "\n")
	tpl := `<h4><small>最近回复</small></h4>
			<hr>
			` + s +
		`<h4><small>被at</small></h4>
			<hr>
			` + a
	return template.HTML(tpl)
}

func (u *Utils) Truncate(html template.HTML, length int) string {
	text := webhelpers.RemoveFormatting(string(html))
	return webhelpers.Truncate(text, length, "...")
}

func (u *Utils) HTML(str string) template.HTML {
	return template.HTML(str)
}

// \n => <br>
func (u *Utils) Br(str string) template.HTML {
	return template.HTML(strings.Replace(str, "\n", "<br>", -1))
}

func (u *Utils) RenderInput(form wtforms.Form, fieldStr string, inputAttrs ...string) template.HTML {
	field, err := form.Field(fieldStr)
	if err != nil {
		panic(err)
	}

	errorClass := ""

	if field.HasErrors() {
		errorClass = " has-error"
	}

	format := `<div class="form-group%s">
        %s
        %s
        %s
    </div>`

	var inputAttrs2 []string = []string{`class="form-control"`}
	inputAttrs2 = append(inputAttrs2, inputAttrs...)

	return template.HTML(
		fmt.Sprintf(format,
			errorClass,
			field.RenderLabel(),
			field.RenderInput(inputAttrs2...),
			field.RenderErrors()))
}

func (u *Utils) RenderInputH(form wtforms.Form, fieldStr string, labelWidth, inputWidth int, inputAttrs ...string) template.HTML {
	field, err := form.Field(fieldStr)
	if err != nil {
		panic(err)
	}

	errorClass := ""

	if field.HasErrors() {
		errorClass = " has-error"
	}
	format := `<div class="form-group%s">
        %s
        <div class="col-lg-%d">
            %s%s
        </div>
    </div>`
	labelClass := fmt.Sprintf(`class="col-lg-%d control-label"`, labelWidth)

	var inputAttrs2 []string = []string{`class="form-control"`}
	inputAttrs2 = append(inputAttrs2, inputAttrs...)

	return template.HTML(
		fmt.Sprintf(format,
			errorClass,
			field.RenderLabel(labelClass),
			inputWidth,
			field.RenderInput(inputAttrs2...),
			field.RenderErrors(),
		))
}

func (u *Utils) HasAd(position string) bool {
	c := DB.C(ADS)
	count, _ := c.Find(bson.M{"position": position}).Limit(1).Count()
	return count == 1
}

func (u *Utils) AdCode(position string) template.HTML {
	c := DB.C(ADS)
	var ad AD
	c.Find(bson.M{"position": position}).Limit(1).One(&ad)

	return template.HTML(ad.Code)
}

func (u *Utils) AssertUser(i interface{}) *User {
	v, _ := i.(User)
	return &v
}

func (u *Utils) AssertNode(i interface{}) *Node {
	v, _ := i.(Node)
	return &v
}

func (u *Utils) AssertTopic(i interface{}) *Topic {
	v, _ := i.(Topic)
	return &v
}

func (u *Utils) AssertArticle(i interface{}) *Article {
	v, _ := i.(Article)
	return &v
}

func (u *Utils) AssertPackage(i interface{}) *Package {
	v, _ := i.(Package)
	return &v
}

func message(w http.ResponseWriter, r *http.Request, title string, message string, class string) {
	renderTemplate(w, r, "message.html", BASE, map[string]interface{}{"title": title, "message": template.HTML(message), "class": class})
}

// 获取链接的页码，默认"?p=1"这种类型
func Page(r *http.Request) (int, error) {
	p := r.FormValue("p")
	page := 1

	if p != "" {
		var err error
		page, err = strconv.Atoi(p)

		if err != nil {
			return 0, err
		}
	}

	return page, nil
}

// 检查一个string元素是否在数组里面
func stringInArray(a []string, x string) bool {
	sort.Strings(a)
	index := sort.SearchStrings(a, x)

	if index == 0 {
		if a[0] == x {
			return true
		}

		return false
	} else if index > len(a)-1 {
		return false
	}

	return true
}

func init() {
	if Config.DB == "" {
		fmt.Println("数据库地址还没有配置,请到config.json内配置db字段.")
		os.Exit(1)
	}

	session, err := mgo.Dial(Config.DB)
	if err != nil {
		fmt.Println("MongoDB连接失败:", err.Error())
		os.Exit(1)
	}

	session.SetMode(mgo.Monotonic, true)

	DB = session.DB("gopher")

	store = sessions.NewCookieStore([]byte(Config.CookieSecret))

	utils = &Utils{}

	// 如果没有status,创建
	var status Status
	c := DB.C(STATUS)
	err = c.Find(nil).One(&status)

	if err != nil {
		c.Insert(&Status{
			Id_:        bson.NewObjectId(),
			UserCount:  0,
			TopicCount: 0,
			ReplyCount: 0,
			UserIndex:  0,
		})
	}

	// 检查是否有超级账户设置
	var superusers []string
	for _, username := range strings.Split(Config.Superusers, ",") {
		username = strings.TrimSpace(username)
		if username != "" {
			superusers = append(superusers, username)
		}
	}

	if len(superusers) == 0 {
		fmt.Println("你没有设置超级账户,请在config.json中的superusers中设置,如有多个账户,用逗号分开")
	}

	c = DB.C(USERS)
	var users []User
	c.Find(bson.M{"issuperuser": true}).All(&users)

	// 如果mongodb中的超级用户不在配置文件中,取消超级用户
	for _, user := range users {
		if !stringInArray(superusers, user.Username) {
			c.Update(bson.M{"_id": user.Id_}, bson.M{"$set": bson.M{"issuperuser": false}})
		}
	}

	// 设置超级用户
	for _, username := range superusers {
		c.Update(bson.M{"username": username, "issuperuser": false}, bson.M{"$set": bson.M{"issuperuser": true}})
	}
}

func staticHandler(templateFile string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		renderTemplate(w, r, templateFile, BASE, map[string]interface{}{})
	}
}

func getPage(r *http.Request) (page int, err error) {
	p := r.FormValue("p")
	page = 1

	if p != "" {
		page, err = strconv.Atoi(p)

		if err != nil {
			return
		}
	}

	return
}

//mark gga
//提取评论中被at的用户名
func findAts(content string) []string {
	regAt := regexp.MustCompile(`@(\S*) `)
	allAts := regAt.FindAllStringSubmatch(content, -1)
	var users []string
	for _, v := range allAts {
		users = append(users, v[1])
	}
	return users
}

// URL: /comment/{contentId}
// 评论，不同内容共用一个评论方法
func commentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		return
	}

	user, _ := currentUser(r)

	vars := mux.Vars(r)
	contentId := vars["contentId"]

	var temp map[string]interface{}
	c := DB.C(CONTENTS)
	c.Find(bson.M{"_id": bson.ObjectIdHex(contentId)}).One(&temp)

	temp2 := temp["content"].(map[string]interface{})
	var contentCreator bson.ObjectId
	contentCreator = temp2["createdby"].(bson.ObjectId)
	type_ := temp2["type"].(int)

	var url string
	switch type_ {
	case TypeArticle:
		url = "/a/" + contentId
	case TypeTopic:
		url = "/t/" + contentId
	case TypePackage:
		url = "/p/" + contentId
	}

	c.Update(bson.M{"_id": bson.ObjectIdHex(contentId)}, bson.M{"$inc": bson.M{"content.commentcount": 1}})

	content := r.FormValue("content")

	html := r.FormValue("html")
	html = strings.Replace(html, "<pre>", `<pre class="prettyprint linenums">`, -1)

	Id_ := bson.NewObjectId()
	now := time.Now()

	c = DB.C(COMMENTS)
	c.Insert(&Comment{
		Id_:       Id_,
		Type:      type_,
		ContentId: bson.ObjectIdHex(contentId),
		Markdown:  content,
		Html:      template.HTML(html),
		CreatedBy: user.Id_,
		CreatedAt: now,
	})

	if type_ == TypeTopic {
		// 修改最后回复用户Id和时间
		c = DB.C(CONTENTS)
		c.Update(bson.M{"_id": bson.ObjectIdHex(contentId)}, bson.M{"$set": bson.M{"latestreplierid": user.Id_.Hex(), "latestrepliedat": now}})

		// 修改中的回复数量
		c = DB.C(STATUS)
		c.Update(nil, bson.M{"$inc": bson.M{"replycount": 1}})
		/*mark ggaaooppeenngg*/
		//修改用户的最近回复
		c = DB.C(USERS)
		//查找评论中at的用户,并且更新recentAts
		users := findAts(content)
		for _, v := range users {
			var user User
			err := c.Find(bson.M{"username": v}).One(&user)
			if err != nil {
				fmt.Println(err)
			} else {
				user.RecentAts = append(user.RecentAts, contentId)
				if err = c.Update(bson.M{"username": user.Username}, bson.M{"$set": bson.M{"recentats": user.RecentAts}}); err != nil {
					fmt.Println(err)
				}
			}
		}

		//修改用户的最近回复
		//该最近回复提醒通过url被点击的时候会被disactive
		//更新最近的评论
		//自己的评论就不提示了
		if contentCreator.Hex() != user.Id_.Hex() {
			var recentreplies []string
			var Creater User
			err := c.Find(bson.M{"_id": contentCreator}).One(&Creater)
			if err != nil {
				fmt.Println(err)
			}
			recentreplies = Creater.RecentReplies
			//添加最近评论所在的主题id
			recentreplies = append(recentreplies, contentId)
			if err = c.Update(bson.M{"_id": contentCreator}, bson.M{"$set": bson.M{"recentreplies": recentreplies}}); err != nil {
				fmt.Println(err)
			}
		}
	}

	http.Redirect(w, r, url, http.StatusFound)
}

// URL: /comment/{commentId}/delete
// 删除评论
func deleteCommentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var commentId string = vars["commentId"]

	c := DB.C(COMMENTS)
	var comment Comment
	err := c.Find(bson.M{"_id": bson.ObjectIdHex(commentId)}).One(&comment)

	if err != nil {
		message(w, r, "评论不存在", "该评论不存在", "error")
		return
	}

	c.Remove(bson.M{"_id": comment.Id_})

	c = DB.C(CONTENTS)
	c.Update(bson.M{"_id": comment.ContentId}, bson.M{"$inc": bson.M{"content.commentcount": -1}})

	if comment.Type == TypeTopic {
		var topic Topic
		c.Find(bson.M{"_id": comment.ContentId}).One(&topic)
		if topic.LatestReplierId == comment.CreatedBy.Hex() {
			if topic.CommentCount == 0 {
				// 如果删除后没有回复，设置最后回复id为空，最后回复时间为创建时间
				c.Update(bson.M{"_id": topic.Id_}, bson.M{"$set": bson.M{"latestreplierid": "", "latestrepliedat": topic.CreatedAt}})
			} else {
				// 如果删除的是该主题最后一个回复，设置主题的最新回复id，和时间
				var latestComment Comment
				c = DB.C("comments")
				c.Find(bson.M{"contentid": topic.Id_}).Sort("-createdat").Limit(1).One(&latestComment)

				c = DB.C("contents")
				c.Update(bson.M{"_id": topic.Id_}, bson.M{"$set": bson.M{"latestreplierid": latestComment.CreatedBy.Hex(), "latestrepliedat": latestComment.CreatedAt}})
			}
		}

		// 修改中的回复数量
		c = DB.C(STATUS)
		c.Update(nil, bson.M{"$inc": bson.M{"replycount": -1}})
	}

	var url string
	switch comment.Type {
	case TypeArticle:
		url = "/a/" + comment.ContentId.Hex()
	case TypeTopic:
		url = "/t/" + comment.ContentId.Hex()
	case TypePackage:
		url = "/p/" + comment.ContentId.Hex()
	}

	http.Redirect(w, r, url, http.StatusFound)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	p := r.FormValue("p")
	page := 1

	if p != "" {
		var err error
		page, err = strconv.Atoi(p)

		if err != nil {
			message(w, r, "页码错误", "页码错误", "error")
			return
		}
	}

	q := r.FormValue("q")

	keywords := strings.Split(q, " ")

	var noSpaceKeywords []string

	for _, keyword := range keywords {
		temp := strings.TrimSpace(keyword)
		if temp != "" {
			noSpaceKeywords = append(noSpaceKeywords, temp)
		}
	}

	var titleConditions []bson.M
	var markdownConditions []bson.M

	for _, keyword := range noSpaceKeywords {
		titleConditions = append(titleConditions, bson.M{"content.title": bson.M{"$regex": bson.RegEx{keyword, "i"}}})
		markdownConditions = append(markdownConditions, bson.M{"content.markdown": bson.M{"$regex": bson.RegEx{keyword, "i"}}})
	}

	c := DB.C(CONTENTS)

	var pagination *Pagination

	if len(noSpaceKeywords) == 0 {
		pagination = NewPagination(c.Find(bson.M{"content.type": TypeTopic}).Sort("-latestrepliedat"), "/search?"+q, PerPage)
	} else {
		pagination = NewPagination(c.Find(bson.M{"$and": []bson.M{
			bson.M{"content.type": TypeTopic},
			bson.M{"$or": []bson.M{
				bson.M{"$and": titleConditions},
				bson.M{"$and": markdownConditions},
			},
			},
		}}).Sort("-latestrepliedat"), "/search?q="+q, PerPage)
	}

	var topics []Topic

	query, err := pagination.Page(page)
	if err != nil {
		message(w, r, "页码错误", "页码错误", "error")
		return
	}

	query.All(&topics)

	if err != nil {
		println(err.Error())
	}

	renderTemplate(w, r, "search.html", BASE, map[string]interface{}{
		"q":          q,
		"topics":     topics,
		"pagination": pagination,
		"page":       page,
		"active":     "topic",
	})
}
