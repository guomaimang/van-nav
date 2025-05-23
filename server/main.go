package main

import (
	"database/sql"
	"embed"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/mereith/nav/goscraper"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
	// _ "github.com/mattn/go-sqlite3"
)

const INDEX = "index.html"

func getIcon(url string) string {
	fmt.Println("getIcon: " + url)
	s, err := goscraper.Scrape(url, 5)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	var result string = ""
	if strings.Contains(s.Preview.Icon, "http:") || strings.Contains(s.Preview.Icon, "https:") {
		result = s.Preview.Icon
	} else {
		//  如果 link 最后一个是 /
		var first string = s.Preview.Link
		var second string = s.Preview.Icon
		if !strings.Contains(s.Preview.Link[len(s.Preview.Link)-1:len(s.Preview.Link)], "/") {
			first = s.Preview.Link + "/"
		}
		// 如果 icon 第一个是 /
		if strings.Contains(s.Preview.Icon[0:1], "/") {
			second = s.Preview.Icon[1:len(s.Preview.Icon)]
		}
		result = first + second
	}
	fmt.Println(result)
	return result
}

func updateCatelog(data updateCatelogDto, db *sql.DB) {

	// 查询分类原名称
	sql_select_old_catelog_name := `select name from nav_catelog where id = ?;`
	var oldName string
	err := db.QueryRow(sql_select_old_catelog_name, data.Id).Scan(&oldName)
	checkErr(err)
	fmt.Println(oldName)

	// 开启事务
	tx, err := db.Begin()
	checkErr(err)

	// 更新分类新名称
	sql_update_catelog := `
		UPDATE nav_catelog
		SET name = ?, sort = ?, hide = ?
		WHERE id = ?;
		`
	stmt, err := tx.Prepare(sql_update_catelog)
	checkTxErr(err, tx)
	res, err := stmt.Exec(data.Name, data.Sort, data.Hide, data.Id)
	checkTxErr(err, tx)
	_, err = res.RowsAffected()
	checkTxErr(err, tx)
	// fmt.Println(affect)

	// 更新工具分类新名称
	sql_update_tools := `
		UPDATE nav_table
		SET catelog = ?
		WHERE catelog = ?;
		`
	stmt2, err := tx.Prepare(sql_update_tools)
	checkTxErr(err, tx)
	res2, err := stmt2.Exec(data.Name, oldName)
	checkTxErr(err, tx)
	_, err = res2.RowsAffected()
	checkTxErr(err, tx)
	// 提交事务
	err = tx.Commit()
	checkErr(err)
}

func getImgFromDB(url1 string, db *sql.DB) Img {
	urlEncoded := url.QueryEscape(url1)
	sql_get_img := `
		SELECT id,url,value FROM nav_img
		WHERE url=?;
		`
	rows, err := db.Query(sql_get_img, urlEncoded)
	checkErr(err)
	var result Img
	var has bool = false
	for rows.Next() {
		err = rows.Scan(&result.Id, &result.Url, &result.Value)
		checkErr(err)
		has = true
	}
	if !has {
		var nullImg string
		l := strings.Split(url1, ".")
		suffix := l[len(l)-1]
		if strings.Contains(suffix, "svg") {
			nullImg = "PD94bWwgdmVyc2lvbj0iMS4wIiBzdGFuZGFsb25lPSJubyI/PjwhRE9DVFlQRSBzdmcgUFVCTElDICItLy9XM0MvL0RURCBTVkcgMS4xLy9FTiIgImh0dHA6Ly93d3cudzMub3JnL0dyYXBoaWNzL1NWRy8xLjEvRFREL3N2ZzExLmR0ZCI+PHN2ZyB0PSIxNjQ5OTM0MTE0OTg2IiBjbGFzcz0iaWNvbiIgdmlld0JveD0iMCAwIDEwMjQgMTAyNCIgdmVyc2lvbj0iMS4xIiB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHAtaWQ9IjI2NjEiIHhtbG5zOnhsaW5rPSJodHRwOi8vd3d3LnczLm9yZy8xOTk5L3hsaW5rIiB3aWR0aD0iMjAwIiBoZWlnaHQ9IjIwMCI+PGRlZnM+PHN0eWxlIHR5cGU9InRleHQvY3NzIj5AZm9udC1mYWNlIHsgZm9udC1mYW1pbHk6IGZlZWRiYWNrLWljb25mb250OyBzcmM6IHVybCgiLy9hdC5hbGljZG4uY29tL3QvZm9udF8xMDMxMTU4X3U2OXc4eWh4ZHUud29mZjI/dD0xNjMwMDMzNzU5OTQ0IikgZm9ybWF0KCJ3b2ZmMiIpLCB1cmwoIi8vYXQuYWxpY2RuLmNvbS90L2ZvbnRfMTAzMTE1OF91Njl3OHloeGR1LndvZmY/dD0xNjMwMDMzNzU5OTQ0IikgZm9ybWF0KCJ3b2ZmIiksIHVybCgiLy9hdC5hbGljZG4uY29tL3QvZm9udF8xMDMxMTU4X3U2OXc4eWh4ZHUudHRmP3Q9MTYzMDAzMzc1OTk0NCIpIGZvcm1hdCgidHJ1ZXR5cGUiKTsgfQ0KPC9zdHlsZT48L2RlZnM+PHBhdGggZD0iTTUxMiAwQzIyOC40MzA3NjkgMCAwIDIyOC40MzA3NjkgMCA1MTJzMjI4LjQzMDc2OSA1MTIgNTEyIDUxMiA1MTItMjI4LjQzMDc2OSA1MTItNTEyUzc5NS41NjkyMzEgMCA1MTIgMHogbTAgNTkuMDc2OTIzYzEwMi40IDAgMTk2LjkyMzA3NyAzNS40NDYxNTQgMjc1LjY5MjMwOCA5NC41MjMwNzdMMTUzLjYgNzg3LjY5MjMwOGMtNTkuMDc2OTIzLTc0LjgzMDc2OS05NC41MjMwNzctMTczLjI5MjMwOC05NC41MjMwNzctMjc1LjY5MjMwOEM1OS4wNzY5MjMgMjYzLjg3NjkyMyAyNjMuODc2OTIzIDU5LjA3NjkyMyA1MTIgNTkuMDc2OTIzeiBtMCA5MDUuODQ2MTU0Yy0xMDIuNCAwLTE5Ni45MjMwNzctMzUuNDQ2MTU0LTI3NS42OTIzMDgtOTQuNTIzMDc3TDg3MC40IDIzNi4zMDc2OTJjNTkuMDc2OTIzIDc0LjgzMDc2OSA5NC41MjMwNzcgMTczLjI5MjMwOCA5NC41MjMwNzcgMjc1LjY5MjMwOCAwIDI0OC4xMjMwNzctMjA0LjggNDUyLjkyMzA3Ny00NTIuOTIzMDc3IDQ1Mi45MjMwNzd6IiBmaWxsPSIjOTk5OTk5IiBwLWlkPSIyNjYyIj48L3BhdGg+PC9zdmc+"
		} else {
			nullImg = "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"
		}

		return Img{Id: 0, Url: url1, Value: nullImg}
	}

	defer rows.Close()
	return result
}

func updateImg(url1 string, db *sql.DB) {
	// 除了更新工具本身之外，也要更新 img 表
	// 先看有没有，有的话就不管了，没有的话就创建
	urlEncoded := url.QueryEscape(url1)
	// fmt.Println("创建时编码:", urlEncoded)\
	base64ImgValue := getImgBase64FromUrl(url1)
	if base64ImgValue == "" {
		return
	}
	sql_get_img := `
		SELECT * FROM nav_img
		WHERE url = ?;
		`

	rows, err := db.Query(sql_get_img, urlEncoded)
	checkErr(err)
	defer rows.Close()
	if !rows.Next() {
		sql_add_img := `
		INSERT INTO nav_img (url, value)
		VALUES (?, ?);
		`
		stmt, err := db.Prepare(sql_add_img)
		checkErr(err)
		_, err = stmt.Exec(urlEncoded, base64ImgValue)
		checkErr(err)
	}
}

func updateTool(data updateToolDto, db *sql.DB) {
	// 除了更新工具本身之外，也要更新 img 表
	sql_update_tool := `
		UPDATE nav_table
		SET name = ?, url = ?, logo = ?, catelog = ?, desc = ?, sort = ?, hide = ?
		WHERE id = ?;
		`
	stmt, err := db.Prepare(sql_update_tool)
	checkErr(err)
	res, err := stmt.Exec(data.Name, data.Url, data.Logo, data.Catelog, data.Desc, data.Sort, data.Hide, data.Id)
	checkErr(err)
	_, err = res.RowsAffected()
	checkErr(err)
	// 更新 img
	updateImg(data.Logo, db)
	// fmt.Println(affect)
}

func updateSetting(data Setting, db *sql.DB) {
	sql_update_setting := `
		UPDATE nav_setting
		SET favicon = ?, title = ?, govRecord = ?, logo192 = ?, logo512 = ?, hideAdmin = ?, hideGithub = ?, jumpTargetBlank = ?
		WHERE id = ?;
		`

	stmt, err := db.Prepare(sql_update_setting)
	checkErr(err)
	res, err := stmt.Exec(data.Favicon, data.Title, data.GovRecord, data.Logo192, data.Logo512, data.HideAdmin, data.HideGithub, data.JumpTargetBlank, 0)
	checkErr(err)
	_, err = res.RowsAffected()
	checkErr(err)
	// fmt.Println(affect)
}

func addApiTokenInDB(data Token, db *sql.DB) {
	sql_add_api_token := `
		INSERT INTO nav_api_token (id,name,value,disabled)
		VALUES (?,?,?,?);
		`
	// fmt.Println("增加分类：",data)
	stmt, err := db.Prepare(sql_add_api_token)
	checkErr(err)

	res, err := stmt.Exec(data.Id, data.Name, data.Value, data.Disabled)
	checkErr(err)
	_, err = res.LastInsertId()
	checkErr(err)
}

func updateUser(data updateUserDto, db *sql.DB) {
	sql_update_user := `
		UPDATE nav_user
		SET name = ?, password = ?
		WHERE id = ?;
		`
	stmt, err := db.Prepare(sql_update_user)
	checkErr(err)
	res, err := stmt.Exec(data.Name, data.Password, data.Id)
	checkErr(err)
	_, err = res.RowsAffected()
	checkErr(err)
	// fmt.Println(affect)
}

func addCatelog(data addCatelogDto, db *sql.DB) {
	// 先检查重复不重复
	existCatelogs := getAllCatelog(db, true)
	var existCatelogsArr []string
	for _, catelogDto := range existCatelogs {
		existCatelogsArr = append(existCatelogsArr, catelogDto.Name)
	}
	if in(data.Name, existCatelogsArr) {
		return
	}
	sql_add_catelog := `
		INSERT INTO nav_catelog (name, sort, hide)
		VALUES (?, ?, ?);
		`
	// fmt.Println("增加分类：",data)
	stmt, err := db.Prepare(sql_add_catelog)
	checkErr(err)
	res, err := stmt.Exec(data.Name, data.Sort, data.Hide)
	checkErr(err)
	_, err = res.LastInsertId()
	checkErr(err)
	// fmt.Println(id)
}

func addTool(data addToolDto, db *sql.DB) int64 {
	sql_add_tool := `
		INSERT INTO nav_table (name, url, logo, catelog, desc, sort, hide)
		VALUES (?, ?, ?, ?, ?, ?, ?);
		`
	stmt, err := db.Prepare(sql_add_tool)
	checkErr(err)
	res, err := stmt.Exec(data.Name, data.Url, data.Logo, data.Catelog, data.Desc, data.Sort, data.Hide)
	checkErr(err)
	id, err := res.LastInsertId()
	checkErr(err)
	// fmt.Println(id)
	updateImg(data.Logo, db)
	return id
}

func getAllTool(db *sql.DB, showHide bool) []Tool {
	var rows *sql.Rows
	var err error
	if showHide {
		sqlGetAll := `SELECT id,name,url,logo,catelog,desc,sort,hide FROM nav_table order by sort;`
		rows, err = db.Query(sqlGetAll)
		checkErr(err)
	} else {
		sqlGetAll := `SELECT id,name,url,logo,catelog,desc,sort,hide FROM nav_table WHERE hide=? order by sort;`
		rows, err = db.Query(sqlGetAll, false)
		checkErr(err)
	}

	results := make([]Tool, 0)
	checkErr(err)
	for rows.Next() {
		var tool Tool
		var hide interface{}
		var sort interface{}
		err = rows.Scan(&tool.Id, &tool.Name, &tool.Url, &tool.Logo, &tool.Catelog, &tool.Desc, &sort, &hide)
		if hide == nil {
			tool.Hide = false
		} else {
			if hide.(int64) == 0 {
				tool.Hide = false
			} else {
				tool.Hide = true
			}
		}
		if sort == nil {
			tool.Sort = 0
		} else {
			i64 := sort.(int64)
			tool.Sort = int(i64)
		}
		checkErr(err)
		results = append(results, tool)
	}
	defer rows.Close()
	return results
}

func getAllCatelog(db *sql.DB, showHide bool) []Catelog {
	var rows *sql.Rows
	var err error
	if showHide {
		sqlGetAll := `SELECT id,name,sort,hide FROM nav_catelog order by sort;`
		rows, err = db.Query(sqlGetAll)
		checkErr(err)
	} else {
		sqlGetAll := `SELECT id,name,sort,hide FROM nav_catelog WHERE hide=?  order by sort;`
		rows, err = db.Query(sqlGetAll, false)
		checkErr(err)
	}

	results := make([]Catelog, 0)
	for rows.Next() {
		var catelog Catelog
		err = rows.Scan(&catelog.Id, &catelog.Name, &catelog.Sort, &catelog.Hide)
		checkErr(err)
		results = append(results, catelog)
	}
	defer rows.Close()
	return results
}

func generateId() int {
	// 生成一个随机 id
	id := int(time.Now().Unix())
	return id
}

var db *sql.DB

func PathExistsOrCreate(path string) {
	_, err := os.Stat(path)
	if err == nil {
		return
	}
	os.Mkdir(path, os.ModePerm)
}

//go:embed public
var fs embed.FS

type binaryFileSystem struct {
	fs   http.FileSystem
	root string
}

func (b *binaryFileSystem) Open(name string) (http.File, error) {
	// fmt.Println("打开文件",name)
	openPath := path.Join(b.root, name)
	return b.fs.Open(openPath)
}

func (b *binaryFileSystem) Exists(prefix string, filepath string) bool {
	if p := strings.TrimPrefix(filepath, prefix); len(p) < len(filepath) {
		var name string
		if p == "" {
			// fmt.Println("找 index")
			name = path.Join(b.root, p, INDEX)
		} else {
			name = path.Join(b.root, p)
		}
		// 判断
		// fmt.Println("文件是否存在？",name)
		if _, err := b.fs.Open(name); err != nil {
			return false
		}
		return true
	}
	return false
}
func BinaryFileSystem(data embed.FS, root string) *binaryFileSystem {
	fs := http.FS(data)
	return &binaryFileSystem{
		fs,
		root,
	}
}

var port = flag.String("port", "6412", "指定监听端口")

func main() {
	flag.Parse()
	initDB()
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	// 嵌入文件夹

	// public,_ := fs.ReadDir("./public")
	// router.StaticFS("/",http.FS(fs))

	router.GET("/manifest.json", ManifastHanlder)
	router.Use(Serve("/", BinaryFileSystem(fs, "public")))
	// router.Use(static.Serve("/", static.LocalFile("./public", true)))
	api := router.Group("/api")
	{
		// 获取数据的路由
		api.GET("/", GetAllHandler)
		// 获取用户信息

		api.POST("/login", LoginHandler)
		api.GET("/logout", LogoutHandler)
		api.GET("/img", getLogoImgHandler)
		// 管理员用的
		admin := api.Group("/admin")
		admin.Use(JWTMiddleware())
		{
			admin.POST("/apiToken", AddApiTokenHandler)
			admin.DELETE("/apiToken/:id", DeleteApiTokenHandler)
			admin.GET("/all", GetAdminAllDataHandler)

			admin.GET("/exportTools", ExportToolsHandler)

			admin.POST("/importTools", ImportToolsHandler)

			admin.PUT("/user", UpdateUserHandler)

			admin.PUT("/setting", UpdateSettingHandler)

			admin.POST("/tool", AddToolHandler)
			admin.DELETE("/tool/:id", DeleteToolHandler)
			admin.PUT("/tool/:id", UpdateToolHandler)

			admin.POST("/catelog", AddCatelogHandler)
			admin.DELETE("/catelog/:id", DeleteCatelogHandler)
			admin.PUT("/catelog/:id", UpdateCatelogHandler)

			// 白名单IP相关路由
			admin.GET("/whiteip", GetWhiteIPHandler)
			admin.POST("/whiteip", AddWhiteIPHandler)
			admin.DELETE("/whiteip/:id", DeleteWhiteIPHandler)
		}
	}
	fmt.Printf("应用启动成功，网址: http://localhost:%s", *port)
	listen := fmt.Sprintf(":%s", *port)
	router.Run(listen)
}

func importTools(data []Tool) {
	var catelogs []string
	for _, v := range data {
		// if ()
		if !in(v.Catelog, catelogs) {
			catelogs = append(catelogs, v.Catelog)
		}
		sql_add_tool := `
			INSERT INTO nav_table (id, name, catelog, url, logo, desc)
			VALUES (?, ?, ?, ?, ?, ?);
			`
		stmt, err := db.Prepare(sql_add_tool)
		checkErr(err)
		res, err := stmt.Exec(v.Id, v.Name, v.Catelog, v.Url, v.Logo, v.Desc)
		checkErr(err)
		_, err = res.LastInsertId()
		checkErr(err)
	}
	for _, catelog := range catelogs {
		var addCatelogDto addCatelogDto
		addCatelogDto.Name = catelog
		addCatelog(addCatelogDto, db)
	}
	// 转存所有图片,异步
	go func(data []Tool, db *sql.DB) {
		for _, v := range data {
			updateImg(v.Logo, db)
		}
	}(data, db)

}

func getSetting(db *sql.DB) Setting {
	sql_get_user := `
		SELECT id,favicon,title,govRecord,logo192,logo512,hideAdmin,hideGithub,jumpTargetBlank FROM nav_setting WHERE id = ?;
		`
	var setting Setting
	row := db.QueryRow(sql_get_user, 0)
	// 建立一个空变量
	var hideGithub interface{}
	var hideAdmin interface{}
	var jumpTargetBlank interface{}
	err := row.Scan(&setting.Id, &setting.Favicon, &setting.Title, &setting.GovRecord, &setting.Logo192, &setting.Logo512, &hideAdmin, &hideGithub, &jumpTargetBlank)
	if err != nil {
		return Setting{
			Id:              0,
			Favicon:         "favicon.ico",
			Title:           "Van Nav",
			GovRecord:       "",
			Logo192:         "logo192.png",
			Logo512:         "logo512.png",
			HideAdmin:       false,
			HideGithub:      false,
			JumpTargetBlank: true,
		}
	}
	if hideGithub == nil {
		setting.HideGithub = false
	} else {
		if hideGithub.(int64) == 0 {
			setting.HideGithub = false
		} else {
			setting.HideGithub = true
		}
	}
	if hideAdmin == nil {
		setting.HideAdmin = false
	} else {
		if hideAdmin.(int64) == 0 {
			setting.HideAdmin = false
		} else {
			setting.HideAdmin = true
		}
	}

	if jumpTargetBlank == nil {
		setting.JumpTargetBlank = true
	} else {
		if jumpTargetBlank.(int64) == 0 {
			setting.JumpTargetBlank = false
		} else {
			setting.JumpTargetBlank = true
		}
	}

	return setting
}

func getApiTokens(db *sql.DB) []Token {
	sql_get_api_tokens := `
		SELECT id,name,value,disabled FROM nav_api_token WHERE disabled = 0;
		`
	results := make([]Token, 0)
	rows, err := db.Query(sql_get_api_tokens)
	checkErr(err)
	for rows.Next() {
		var token Token
		err = rows.Scan(&token.Id, &token.Name, &token.Value, &token.Disabled)
		checkErr(err)
		results = append(results, token)
	}
	defer rows.Close()
	return results
}

func getUser(name string, db *sql.DB) User {
	sql_get_user := `
		SELECT id,name,password FROM nav_user WHERE name = ?;
		`
	var user User
	row := db.QueryRow(sql_get_user, name)
	err := row.Scan(&user.Id, &user.Name, &user.Password)
	checkErr(err)
	return user
}
