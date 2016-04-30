package main

import "fmt"

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jinzhu/gorm"

	_ "github.com/lib/pq"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"

	"github.com/YagoCarballo/kumquat.academy.api/tools"
	"github.com/YagoCarballo/kumquat.academy.api/database/models"
	"time"
	"io"
)

const BASE_PATH = "./"

type Header struct {
	Title       string
	Description string
	Author      string
}

type Installer struct {
	Intro         string
	DatabaseTypes []string
	Header        *Header
}

var templates *template.Template

func init() {
	// initialize the templates,
	// couldn't have used http://golang.org/pkg/html/template/#ParseGlob
	// since we have custom delimiters.
	basePath := BASE_PATH + "templates/"
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// don't process folders themselves
		if info.IsDir() {
			return nil
		}
		templateName := path[len(basePath):]
		if templates == nil {
			templates = template.New(templateName)
			_, err = templates.ParseFiles(path)
		} else {
			_, err = templates.New(templateName).ParseFiles(path)
		}
		log.Printf("Processed template %s\n", templateName)
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
}

func installHandler(w http.ResponseWriter, r *http.Request) {
	headerObj := Header{
		Title:       "Kumquat Academy - Installer",
		Description: "Install Asistant for the Kumquat Academy - Learning Platform",
		Author:      "Yago Carballo",
	}

	passedObj := Installer{
		Intro:         "The following steps will help you set up the platform in your own server.",
		DatabaseTypes: []string{"MySQL"}, // Future support for: sqlite3, foundation, microsoft-sql and postgres
		Header:        &headerObj,
	}

	templates.ExecuteTemplate(w, "header", headerObj)
	templates.ExecuteTemplate(w, "installPage", passedObj)
}

func installFinishedHandler(w http.ResponseWriter, r *http.Request) {
	headerObj := Header{
		Title:       "Kumquat Academy - Installer",
		Description: "Installation finished",
		Author:      "Yago Carballo",
	}

	passedObj := Installer{
		Intro:         "Installation finished.",
		DatabaseTypes: []string{"MySQL"},
		Header:        &headerObj,
	}

	templates.ExecuteTemplate(w, "header", headerObj)
	templates.ExecuteTemplate(w, "installFinishPage", passedObj)
}

func parseSettings(r *http.Request) (*tools.Settings, bool, bool) {
	title := r.Form.Get("page-title")
	description := r.Form.Get("page-description")
	serverPortRaw := r.Form.Get("server-port")
	dbName := r.Form.Get("db-name")
	dbHost := r.Form.Get("db-host")
	dbPortRaw := r.Form.Get("db-port")
	dbUsername := r.Form.Get("db-username")
	dbPassword := r.Form.Get("db-password")
	dbCreate := r.Form.Get("db-create")
	dbDemo := r.Form.Get("db-demo")

	// Sets the default ports (In case the provided ones can't be parsed)
	serverPort := 3000
	dbPort := 3306

	// Parses the Server Port
	if serverPortRaw != "" {
		parsedServerPort, err := strconv.Atoi(serverPortRaw)
		if err != nil {
			log.Println("Invalid Database port, falling back to 3000")
		} else {
			serverPort = parsedServerPort
		}
	}

	// Parses the Database Port
	if dbPortRaw != "" {
		parsedDBPort, err := strconv.Atoi(dbPortRaw)
		if err != nil {
			log.Println("Invalid Database port, falling back to 3306")
		} else {
			dbPort = parsedDBPort
		}
	}

	// Creates the DB Host
	dbUrl := fmt.Sprintf("%s:%d", dbHost, dbPort)

	// Returns the Settings Object
	return &tools.Settings{
		Title:       title,
		Description: description,
		Database: tools.Database{
			Type:	  "MySQL",
			Mysql:	  tools.MySQL{
				Username: dbUsername,
				Password: dbPassword,
				Host:     dbUrl,
				Name:     dbName,
			},
			Sqlite:	  tools.SQLite{
				Path:	  "",
			},
		},
		Server: tools.Server{
			Port:  serverPort,
			Debug: false,
			PrivateKey: "./privateKey.pem",
			PublicKey: "./publicKey.pub",
			UploadsPath: "./attachments",
		},
		Api: tools.Api{
			Prefix:  "/api",
			Version: 1,
		},
	}, (dbCreate == "on"), (dbDemo == "on")
}

func doInstallHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	// Initializes the Settings Object.
	settings, dbCreate, dbDemo := parseSettings(req)

	// Creates the settings.toml file with the new settings.
	settings.Save()

	// Connects to the Database
	var database *sql.DB
	var mysqlError, mysqlDownError error

	dbSettings := settings.Database

	uri := fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?charset=utf8&parseTime=True&loc=Local",
		dbSettings.Mysql.Username,
		dbSettings.Mysql.Password,
		dbSettings.Mysql.Host,
		dbSettings.Mysql.Name,
	)

	// Connects to the Database
	if database, mysqlError = sql.Open("mysql", uri); mysqlError != nil {
		log.Println(mysqlError)
		return
	}

	// Open doesn't open a connection. Validate DSN data:
	mysqlDownError = database.Ping()
	if mysqlDownError != nil {
		log.Println(mysqlDownError)
		return
	}

	if dbCreate {
		db, err := gorm.Open("mysql", uri)
		if err != nil {
			log.Fatal(err)
		}

		// Check if DB is reachable
		err = db.DB().Ping()
		if err != nil {
			log.Fatal(err)
		}

		// Logs the DB Session
		log.Printf("Connected to MySQL { server: %s, db: %s }", dbSettings.Mysql.Host, dbSettings.Mysql.Name)

		//db.LogMode(true)

		// Creates or Migrates the user's table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.User{})

		// Creates / Migrates table Session
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.Session{})
		db.Model(&models.Session{}).AddForeignKey("user_id", "users(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the course's table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.Course{})

		// Creates or Migrates the class's table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.Class{})
		db.Model(&models.Class{}).AddUniqueIndex("idx_class_course_title", "course_id", "title")
		db.Model(&models.Class{}).AddForeignKey("course_id", "courses(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the course levels table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.CourseLevel{})
		db.Exec("ALTER TABLE course_levels ADD CONSTRAINT fk_courseLevels_classes FOREIGN KEY (class_id, course_id) REFERENCES classes(id, course_id);")

		// Creates or Migrates the module's table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.Module{})

		// Creates or Migrates the user role's table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.Role{})

		// Creates or Migrates the level modules table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.LevelModule{})
		db.Exec("ALTER TABLE level_modules ADD CONSTRAINT fk_levelModules_courseLevels_course_class FOREIGN KEY (level, class_id) REFERENCES course_levels(level, class_id);")
		db.Model(&models.LevelModule{}).AddForeignKey("module_id", "modules(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the user modules's table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.UserModule{})
		db.Model(&models.UserModule{}).AddForeignKey("user_id", "users(id)", "RESTRICT", "RESTRICT")
		db.Model(&models.UserModule{}).AddForeignKey("module_code", "level_modules(code)", "RESTRICT", "RESTRICT")
		db.Model(&models.UserModule{}).AddForeignKey("role_id", "roles(id)", "RESTRICT", "RESTRICT")
		db.Model(&models.UserModule{}).AddForeignKey("class_id", "classes(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the user courses's table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.UserCourse{})
		db.Model(&models.UserCourse{}).AddForeignKey("user_id", "users(id)", "RESTRICT", "RESTRICT")
		db.Model(&models.UserCourse{}).AddForeignKey("course_id", "courses(id)", "RESTRICT", "RESTRICT")
		db.Model(&models.UserCourse{}).AddForeignKey("role_id", "roles(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the attachments table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.Attachment{})

		// Creates or Migrates the assignments table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.Assignment{})
		db.Model(&models.Assignment{}).AddForeignKey("module_code", "level_modules(code)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the exams table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.Exam{})
		db.Model(&models.Exam{}).AddForeignKey("module_code", "level_modules(code)", "RESTRICT", "RESTRICT")
		db.Model(&models.Exam{}).AddForeignKey("attachment_id", "attachments(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the Pages table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.Page{})
		db.Model(&models.Page{}).AddForeignKey("module_id", "modules(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the Lecture Slot table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.LectureSlot{})
		db.Model(&models.LectureSlot{}).AddForeignKey("module_id", "modules(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the Lectures table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.Lecture{})
		db.Model(&models.Lecture{}).AddForeignKey("module_id", "modules(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the Materials table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.Materials{})
		db.Model(&models.Materials{}).AddForeignKey("module_id", "modules(id)", "RESTRICT", "RESTRICT")
		db.Model(&models.Materials{}).AddForeignKey("lecture_id", "lectures(id)", "RESTRICT", "RESTRICT")
		db.Model(&models.Materials{}).AddForeignKey("attachment_id", "attachments(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the Submissions table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.Submission{})
		db.Model(&models.Submission{}).AddForeignKey("user_id", "users(id)", "RESTRICT", "RESTRICT")
		db.Model(&models.Submission{}).AddForeignKey("assignment_id", "assignments(id)", "RESTRICT", "RESTRICT")
		db.Model(&models.Submission{}).AddForeignKey("attachment_id", "attachments(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the Submissions table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.StudentExam{})
		db.Model(&models.StudentExam{}).AddForeignKey("user_id", "users(id)", "RESTRICT", "RESTRICT")
		db.Model(&models.StudentExam{}).AddForeignKey("exam_id", "exams(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the Announcements table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.Announcement{})
		db.Model(&models.Announcement{}).AddForeignKey("user_id", "users(id)", "RESTRICT", "RESTRICT")
		db.Model(&models.Announcement{}).AddForeignKey("module_id", "modules(id)", "RESTRICT", "RESTRICT")
		db.Model(&models.Announcement{}).AddForeignKey("assignment_id", "assignments(id)", "RESTRICT", "RESTRICT")
		db.Model(&models.Announcement{}).AddForeignKey("course_id", "courses(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the Teams table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.Team{})
		db.Model(&models.Team{}).AddForeignKey("assignment_id", "assignments(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the Team Members table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.TeamMember{})
		db.Model(&models.TeamMember{}).AddForeignKey("team_id", "teams(id)", "RESTRICT", "RESTRICT")
		db.Model(&models.TeamMember{}).AddForeignKey("user_id", "users(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the Tasks table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.Task{})
		db.Model(&models.Task{}).AddForeignKey("assignment_id", "assignments(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the Completed Tasks table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.CompletedTask{})
		db.Model(&models.CompletedTask{}).AddForeignKey("user_id", "users(id)", "RESTRICT", "RESTRICT")
		db.Model(&models.CompletedTask{}).AddForeignKey("task_id", "tasks(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the Team Completed Tasks table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.TeamCompletedTask{})
		db.Model(&models.TeamCompletedTask{}).AddForeignKey("team_id", "teams(id)", "RESTRICT", "RESTRICT")
		db.Model(&models.TeamCompletedTask{}).AddForeignKey("task_id", "tasks(id)", "RESTRICT", "RESTRICT")

		// Creates or Migrates the Reset Password table if it does't exist or changed
		db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&models.ResetPassword{})
		db.Model(&models.ResetPassword{}).AddForeignKey("user_id", "users(id)", "RESTRICT", "RESTRICT")
	}

	if dbDemo {
		db, err := gorm.Open("mysql", uri)
		if err != nil {
			log.Fatal(err)
		}

		// Check if DB is reachable
		err = db.DB().Ping()
		if err != nil {
			log.Fatal(err)
		}

		// Get the GMT Timezone to use as base for the Demo Data Dates
		gmt := time.FixedZone("GMT", 0)

		avatars := []models.Attachment{
			models.Attachment{
				ID:		1,
				Name:	"82.jpg",
				Type:	"image/jpg",
				Url:	"1f77fb90-c32b-4de4-804d-a0cb7dde4cd5",
			},
			models.Attachment{
				ID:		2,
				Name:	"62.jpg",
				Type:	"image/jpg",
				Url:	"3abef575-0101-4487-8715-64bf2e430083",
			},
			models.Attachment{
				ID:		3,
				Name:	"11.jpg",
				Type:	"image/jpg",
				Url:	"3b891aae-8ea0-4324-8a3e-b667b5ea23d9",
			},
			models.Attachment{
				ID:		4,
				Name:	"40.jpg",
				Type:	"image/jpg",
				Url:	"dab71f4f-3f65-487b-8f9d-5bacd3d92bc1",
			},
		}

		for _, avatar := range avatars {
			CopyFile("./demoData/" + avatar.Name, "../kumquat.academy.api/attachments/" + avatar.Url)
			db.FirstOrCreate(&avatar, avatar)
		}

		// admin
		// c7ad44cbad762a5da0a452f9e854fdc1e0e7a52a38015f23f3eab1d80b931dd472634dfac71cd34ebc35d16ab7fb8a90c81f975113d6c7538dc69dd8de9077ec
		adminUser := models.User{
			ID: 			1,
			Username:		"admin",
			Password:		"$2a$10$1rqCHXRQ1h0se3jnJO5ZtuX5keEQOTPL1Tkb4W4yEAcV0x26l7KEO",
			Email:			"jane.johnston68@example.com",
			FirstName:		"Jane",
			LastName:		"Johnston",
			DateOfBirth:	time.Date(1970, 2, 9, 0, 0, 0, 0, gmt),
			MatricNumber:	"000000000",
			MatricDate:		time.Now().In(gmt),
			Active:			true,
			Admin:			true,
			AvatarId:		avatars[0].ID,
		}

		// teacher
		// 50ecc45020be014e68d714cd076007e84a9621d9a5e589a916e45273014830b399d143a57f525554bfe9e751d97fe0fa884dbdea7b07721723b4eff39e9d28ad
		teacherUser := models.User{
			ID: 			2,
			Username:		"teacher",
			Password:		"$2a$10$xiu4.QS1oUOtlsgJdbdZsu4nDLGUfRfRKLdvjsxK4RjNrnhoZbFI6",
			Email:			"eugene.ward72@example.com",
			FirstName:		"Eugene",
			LastName:		"Ward",
			DateOfBirth:	time.Date(1985, 2, 6, 0, 0, 0, 0, gmt),
			MatricNumber:	"111111111",
			MatricDate:		time.Now().In(gmt),
			Active:			true,
			Admin:			false,
			AvatarId:		avatars[1].ID,
		}

		// student
		// 32ade5e7c36fa329ea39dbc352743db40da5aa7460ec55f95b999d6371ad20170094d88d9296643f192e9d5433b8d6d817d6777632e556e96e58f741dc5b3550
		studentUser := models.User{
			ID: 			3,
			Username:		"student",
			Password:		"$2a$10$/TVggaU5mgv103DU3w1FruWKesYujzOtIjy6ik0fQ6jPGAiSkHiA.",
			Email:			"anna.matthews10@example.com",
			FirstName:		"Anna",
			LastName:		"Matthews",
			DateOfBirth:	time.Date(1976, 4, 6, 0, 0, 0, 0, gmt),
			MatricNumber:	"222222222",
			MatricDate:		time.Now().In(gmt),
			Active:			true,
			Admin:			false,
			AvatarId:		avatars[2].ID,
		}

		// guest
		// b0e0ec7fa0a89577c9341c16cff870789221b310a02cc465f464789407f83f377a87a97d635cac2666147a8fb5fd27d56dea3d4ceba1fc7d02f422dda6794e3c
		guestUser := models.User{
			ID: 			4,
			Username:		"guest",
			Password:		"$2a$10$ouCsus6K//.Xr04sNS0M9O1s8BXEDHdC9pFupCCup.leWdSlPn9hm",
			Email:			"rick.peters60@example.com",
			FirstName:		"Rick",
			LastName:		"Peters",
			DateOfBirth:	time.Date(1974, 2, 10, 0, 0, 0, 0, gmt),
			MatricNumber:	"333333333",
			MatricDate:		time.Now().In(gmt),
			Active:			true,
			Admin:			false,
			AvatarId:		avatars[3].ID,
		}

		db.FirstOrCreate(&adminUser, adminUser)
		db.FirstOrCreate(&studentUser, studentUser)
		db.FirstOrCreate(&teacherUser, teacherUser)
		db.FirstOrCreate(&guestUser, guestUser)

		session := models.Session{
			Token:		"a077c80d-77e2-4328-80c4-f2b4ccf995c4",
			UserID:		1,
			DeviceID:	"-Test-Device-",
			ExpiresIn:	time.Now().In(gmt).AddDate(0, 0, 7),
			CreatedOn:	time.Now().In(gmt),
		}

		db.FirstOrCreate(&session, session)

		courses := []models.Course{
			models.Course{
				ID:				1,
				Title:			"BSc (Hons) Applied Computing",
				Description:	"Computing",
			},
			models.Course{
				ID:				2,
				Title:			"MA Artificial Intelligence",
				Description:	"AI",
			},
		}

		for _, course := range courses {
			db.FirstOrCreate(&course, course)
		}

		classes := []models.Class{
			models.Class{
				ID:			1,
				CourseID:		1,
				Title:			"2016/2017",
				Start:			time.Now().In(gmt),
				End:			time.Now().In(gmt).AddDate(1, 0, 0),
			},
			models.Class{
				ID:			2,
				CourseID:		2,
				Title:			"2017/2018",
				Start:			time.Now().In(gmt).AddDate(1, 0, 0),
				End:			time.Now().In(gmt).AddDate(2, 0, 0),
			},
		}

		for _, class := range classes {
			db.FirstOrCreate(&class, class)
		}

		courseLevels := []models.CourseLevel{
			models.CourseLevel{
				Level:		1,
				CourseID:	classes[0].CourseID,
				ClassID:	classes[0].ID,
				Start:		time.Now().In(gmt),
				End:		time.Now().In(gmt).AddDate(1, 0, 0),
			},
			models.CourseLevel{
				Level:		2,
				CourseID:	classes[0].CourseID,
				ClassID:	classes[0].ID,
				Start:		time.Now().In(gmt).AddDate(1, 0, 0),
				End:		time.Now().In(gmt).AddDate(2, 0, 0),
			},
			models.CourseLevel{
				Level:		1,
				CourseID:	classes[1].CourseID,
				ClassID:	classes[1].ID,
				Start:		time.Now().In(gmt),
				End:		time.Now().In(gmt).AddDate(1, 0, 0),
			},
		}

		for _, level := range courseLevels {
			db.FirstOrCreate(&level, level)
		}

		modules := []models.Module{
			models.Module{
				ID:				1,
				Title:			"Big Data",
				Color:			"#9C0098",
				Icon:			"fa-cloud",
				Duration:		12,
				Description:	"Introduction to the world of Big Data",
			},
			models.Module{
				ID:				2,
				Title:			"Graphics",
				Color:			"#006099",
				Icon:			"fa-codepen",
				Duration:		5,
				Description:	"3D Computer graphics",
			},
			models.Module{
				ID:				3,
				Title:			"UX",
				Color:			"#009E00",
				Icon:			"fa-eye",
				Duration:		12,
				Description:	"User Experience Design",
			},
		}

		for _, module := range modules {
			db.FirstOrCreate(&module, module)
		}

		levelModules := []models.LevelModule{
			models.LevelModule{
				Code:		"AC31007",
				Level:		1,
				ClassID:	classes[0].ID,
				ModuleID:	modules[0].ID,
				Status:		models.ModuleOngoing,
				Start:		classes[0].Start,
			},
			models.LevelModule{
				Code:		"AC41008",
				Level:		1,
				ClassID:	classes[0].ID,
				ModuleID:	modules[1].ID,
				Status:		models.ModuleOngoing,
				Start:		classes[0].Start,
			},
			models.LevelModule{
				Code:		"AC52001",
				Level:		1,
				ClassID:	classes[1].ID,
				ModuleID:	modules[2].ID,
				Status:		models.ModuleOngoing,
				Start:		classes[0].Start,
			},
			models.LevelModule{
				Code:		"AC22001",
				Level:		2,
				ClassID:	classes[0].ID,
				ModuleID:	modules[2].ID,
				Status:		models.ModuleFuture,
				Start:		classes[0].Start,
			},
		}

		for _, level := range levelModules {
			db.FirstOrCreate(&level, level)
		}

		userRoles := []models.Role{
			models.Role{
				ID:				1,
				Name:			"Admin",
				Description:	"Admin of a module / course.",
				CanRead:			true,
				CanWrite:			true,
				CanDelete:			true,
				CanUpdate:			true,
			},
			models.Role{
				ID:				2,
				Name:			"Lecturer",
				Description:	"Teacher of a module / course.",
				CanRead:			true,
				CanWrite:			true,
				CanDelete:			true,
				CanUpdate:			true,
			},
			models.Role{
				ID:				3,
				Name:			"Student",
				Description:	"Student of a module / course.",
				CanRead:			true,
				CanWrite:			false,
				CanDelete:			false,
				CanUpdate:			false,
			},
		}

		for _, role := range userRoles {
			db.FirstOrCreate(&role, role)
		}

		userModules := []models.UserModule{
			models.UserModule{UserID: teacherUser.ID, ModuleCode: levelModules[0].Code, RoleID: userRoles[1].ID, ClassID: classes[0].ID},
			models.UserModule{UserID: teacherUser.ID, ModuleCode: levelModules[3].Code, RoleID: userRoles[1].ID, ClassID: classes[0].ID},
			models.UserModule{UserID: studentUser.ID, ModuleCode: levelModules[0].Code, RoleID: userRoles[2].ID, ClassID: classes[0].ID},
			models.UserModule{UserID: studentUser.ID, ModuleCode: levelModules[1].Code, RoleID: userRoles[2].ID, ClassID: classes[0].ID},
			models.UserModule{UserID: studentUser.ID, ModuleCode: levelModules[2].Code, RoleID: userRoles[2].ID, ClassID: classes[1].ID},
		}

		for _, userModule := range userModules {
			db.FirstOrCreate(&userModule, userModule)
		}

		assignments := []models.Assignment{
			models.Assignment{
				Title: "Erlang Project",
				Description: `
					<h1>Erlang Project</h1>
					<p>Use erlang to create a concurrent </p>
				`,
				Status: models.AssignmentCreated,
				Weight: 0.20,
				Start: classes[int(levelModules[0].ClassID)].Start,
				End: classes[int(levelModules[0].ClassID)].Start.AddDate(0, 0, int(7 * modules[int(levelModules[0].ModuleID)].Duration)),
				ModuleCode: "AC31007",
			},
			models.Assignment{
				Title: "NoSQL Presentation",
				Description: `
					<h1>NoSQL Presentation</h1>
					<p>Research and create a presentation for your allocated NoSQL Database.</p>
				`,
				Status: models.AssignmentCreated,
				Weight: 0.20,
				Start: classes[int(levelModules[0].ClassID)].Start,
				End: classes[int(levelModules[0].ClassID)].Start.AddDate(0, 0, int(7 * modules[int(levelModules[0].ModuleID)].Duration)),
				ModuleCode: "AC31007",
			},
			models.Assignment{
				Title: "Exam",
				Description: `
					<h1>Exam</h1>
				`,
				Status: models.AssignmentCreated,
				Weight: 0.60,
				Start: classes[int(levelModules[0].ClassID)].End,
				End: classes[int(levelModules[0].ClassID)].End,
				ModuleCode: "AC31007",
			},
		}

		for _, assignment := range assignments {
			db.FirstOrCreate(&assignment, assignment)
		}

		teacherCourses := models.UserCourse{UserID: teacherUser.ID, CourseID: courses[1].ID, RoleID: userRoles[1].ID}
		db.FirstOrCreate(&teacherCourses, teacherCourses)

		baseDate := time.Date(2016, 1, 4, 0, 0, 0, 0, gmt) // Monday
		lectureSlots := []models.LectureSlot{
			models.LectureSlot{
				ID: 		1,
				ModuleID: 	1,
				Location: 	"Seminar Room 2",
				Type: 		"Lecture",
				Start:		baseDate.Add(time.Duration(9) * time.Hour),  // Monday at 9:00
				End:		baseDate.Add(time.Duration(10) * time.Hour), // Monday at 10:00
			},
			models.LectureSlot{
				ID: 		2,
				ModuleID: 	1,
				Location: 	"Dalhousie 2F11",
				Type: 		"Lecture",
				Start:		baseDate.AddDate(0, 0, 2).Add(time.Duration(11) * time.Hour), // Wednesday at 11:00
				End:		baseDate.AddDate(0, 0, 2).Add(time.Duration(13) * time.Hour), // Wednesday at 13:00
			},
			models.LectureSlot{
				ID: 		3,
				ModuleID: 	1,
				Location: 	"QMB Labs 1 & 2",
				Type: 		"Lab",
				Start:		baseDate.AddDate(0, 0, 4).Add(time.Duration(9) * time.Hour), // Friday at 9:00
				End:		baseDate.AddDate(0, 0, 4).Add(time.Duration(13) * time.Hour), // Friday at 13:00
			},
			models.LectureSlot{
				ID: 		4,
				ModuleID: 	2,
				Location: 	"Dalhousie 1G05 (G)",
				Type: 		"Lecture",
				Start:		baseDate.AddDate(0, 0, 1).Add(time.Duration(16) * time.Hour),  // Tuesday at 16:00
				End:		baseDate.AddDate(0, 0, 1).Add(time.Duration(17) * time.Hour), // Tuesday at 17:00
			},
			models.LectureSlot{
				ID: 		5,
				ModuleID: 	2,
				Location: 	"Dalhousie 2F13",
				Type: 		"Lecture",
				Start:		baseDate.AddDate(0, 0, 3).Add(time.Duration(9) * time.Hour), // Thursday at 9:00
				End:		baseDate.AddDate(0, 0, 3).Add(time.Duration(13) * time.Hour), // Thursday at 13:00
			},
		}

		for _, lectureSlot := range lectureSlots {
			db.FirstOrCreate(&lectureSlot, lectureSlot)
		}

		baseDate = time.Now().In(gmt)
		startYear, startWeek := baseDate.ISOWeek()
		baseDate = tools.FirstDayOfISOWeek(startYear, startWeek, gmt) // This Monday
		lectures := []models.Lecture{
			models.Lecture{
				Description:	"<h1>Introduction to Big Data</h1><p>This lecture will show an overview of the module.</p>",
				ModuleID:		lectureSlots[0].ModuleID,
				LectureSlotID:	&lectureSlots[0].ID,
				Location:		lectureSlots[0].Location,
				Topic:			"Introduction to Big Data",
				Start:			baseDate.Add(time.Duration(9) * time.Hour),  // Monday at 9:00
				End:			baseDate.Add(time.Duration(10) * time.Hour), // Monday at 10:00
				Canceled:		false,
			},
			models.Lecture{
				Description:	"<h1>Hadoop</h1><p>This lecture will introduce Hadoop.</p>",
				ModuleID:		lectureSlots[1].ModuleID,
				LectureSlotID:	&lectureSlots[1].ID,
				Location:		lectureSlots[1].Location,
				Topic:			"Hadoop",
				Start:			baseDate.AddDate(0, 0, 2).Add(time.Duration(11) * time.Hour), // Wednesday at 11:00
				End:			baseDate.AddDate(0, 0, 2).Add(time.Duration(13) * time.Hour), // Wednesday at 13:00
				Canceled:		false,
			},
			models.Lecture{
				Description:	"<h1>Erlang</h1><p>In this Lab we will setup Erlang in our computers and run some sample programs.</p>",
				ModuleID:		lectureSlots[2].ModuleID,
				LectureSlotID:	&lectureSlots[2].ID,
				Location:		lectureSlots[2].Location,
				Topic:			"Erlang",
				Start:			baseDate.AddDate(0, 0, 4).Add(time.Duration(9) * time.Hour), // Friday at 9:00
				End:			baseDate.AddDate(0, 0, 4).Add(time.Duration(13) * time.Hour), // Friday at 13:00
				Canceled:		true,
			},
			models.Lecture{
				Description:	"<h1>Introduction to OpenGL</h1><p>In this lecture we will see an overview of the module.</p>",
				ModuleID:		lectureSlots[3].ModuleID,
				LectureSlotID:	&lectureSlots[3].ID,
				Location:		lectureSlots[3].Location,
				Topic:			"Introduction to OpenGL",
				Start:			baseDate.AddDate(0, 0, 1).Add(time.Duration(16) * time.Hour),  // Tuesday at 16:00
				End:			baseDate.AddDate(0, 0, 1).Add(time.Duration(17) * time.Hour), // Tuesday at 17:00
				Canceled:		false,
			},
			models.Lecture{
				Description:	"<h1>Setup OpenGL</h1><p>In this lab we will setup our development environment and run the first sample program.</p>",
				ModuleID:		lectureSlots[4].ModuleID,
				LectureSlotID:	&lectureSlots[4].ID,
				Location:		lectureSlots[4].Location,
				Topic:			"Introduction to OpenGL",
				Start:			baseDate.AddDate(0, 0, 3).Add(time.Duration(9) * time.Hour), // Thursday at 9:00
				End:			baseDate.AddDate(0, 0, 3).Add(time.Duration(13) * time.Hour), // Thursday at 13:00
				Canceled:		false,
			},
		}

		for _, lecture := range lectures {
			db.FirstOrCreate(&lecture, lecture)
		}
	}

	fmt.Println("Installation Finished")

	installFinishedHandler(w, req)
}

func StartInstallServer() {
	// serve static assets showing how to strip/change the path.
	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir(BASE_PATH + "resources/"))))
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir(BASE_PATH + "resources/"))))
	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir(BASE_PATH + "resources/"))))

	// home page handler, defined in handlers.go
	http.HandleFunc("/", installHandler)
	http.HandleFunc("/do-install", doInstallHandler)

	// Sets the default port as 3000
	port := 3000

	// Overrides the Port with the Environment Variable PORT (if available)
	if os.Getenv("PORT") != "" {
		// Get's the PORT Environment Variable and parses it into an Int
		rawPort, err := strconv.Atoi(os.Getenv("PORT"))
		if err != nil {
			log.Println("Invalid Port detected in the Environment Variables, Falling back to Settings")
		} else {
			port = rawPort
		}
	}

	// Logs the Port being Used for Installation
	fmt.Printf("Starting Installer on port: %d\n", port)

	// Starts the Installation Server (Independent from the main Server)
	http.ListenAndServe(":3000", nil)
}

// Starts the Server
func main() {
	StartInstallServer()
}

// Tools, used to copy the avatars into the d
func CopyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	if err = os.Link(src, dst); err == nil {
		return
	}
	err = copyFileContents(src, dst)
	return
}

func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}
