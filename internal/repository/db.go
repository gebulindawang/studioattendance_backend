package repository

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"StudioAttendance_backend/internal/config"

	mysqlconfig "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB() error {

	dsn := mysqlconfig.NewConfig()
	dsn.User = config.DBUser
	dsn.Passwd = config.DBPassword
	dsn.Net = "tcp"
	dsn.Addr = net.JoinHostPort(config.DBHost, config.DBPort)
	dsn.DBName = config.DBName
	dsn.ParseTime = true
	dsn.Loc = config.AttendanceLocation
	dsn.Params = map[string]string{"charset": "utf8mb4"}

	db, err := gorm.Open(mysql.Open(dsn.FormatDSN()), &gorm.Config{
		Logger: logger.New(log.New(os.Stderr, "", log.LstdFlags), logger.Config{
			SlowThreshold: 200 * time.Millisecond, LogLevel: logger.Warn, ParameterizedQueries: true,
		}),
		NowFunc: func() time.Time { return time.Now().In(config.AttendanceLocation) },
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	DB = db
	return nil
}
