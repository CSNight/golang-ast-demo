package db

import (
	"golang-ast/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go.uber.org/zap"
)

type DB struct {
	log *zap.Logger
	orm *gorm.DB
}

func Init(conf *conf.GConfig, log *zap.Logger) (*DB, error) {
	config := gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   NewGormLog(log),
	}
	dbIns, err := gorm.Open(mysql.Open(conf.AppCfg.DbConn), &config)
	if err != nil {
		return nil, err
	}
	dbms := &DB{
		log: log.Named("\u001B[33m[DB]\u001B[0m"),
		orm: dbIns,
	}
	err = dbIns.AutoMigrate(
		&SysUser{}, &SysRole{}, &SysMenu{}, &SysPermission{})
	if err != nil {
		return nil, err
	}
	return dbms, nil
}

func Preload(d *gorm.DB) *gorm.DB {
	return d.Preload(clause.Associations, Preload)
}

type Page struct {
	TotalPages    int64 `json:"total_pages"`
	TotalElements int64 `json:"total_elements"`
	Content       any   `json:"content"`
}
