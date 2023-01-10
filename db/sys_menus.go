package db

import (
	"time"
)

type SysMenu struct {
	Id       int       `json:"id" gorm:"type:int not null;autoIncrement;primaryKey;unique;uniqueIndex"`
	Iframe   bool      `json:"iframe" gorm:"type:tinyint"`
	Name     string    `json:"name" gorm:"type:varchar(255)"`
	Com      string    `json:"com" gorm:"type:varchar(255)"`
	Pid      int       `json:"pid" gorm:"type:int not null"`
	Sort     int       `json:"sort" gorm:"type:int not null"`
	Icon     string    `json:"icon" gorm:"type:varchar(255)"`
	Path     string    `json:"path" gorm:"type:varchar(255)"`
	Hidden   bool      `json:"hidden" gorm:"type:tinyint(1)"`
	ComName  string    `json:"com_name" gorm:"type:varchar(255)"`
	Ct       time.Time `json:"ct" gorm:"type:datetime not null;default:CURRENT_TIMESTAMP"`
	Children []SysMenu `json:"children" gorm:"foreignKey:pid;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type MenuFilter struct {
	Idx    []string `json:"idx" query:"type:in,field:id,omitempty"`
	Name   string   `json:"name" query:"type:in_like,field:name,omitempty"`
	Hidden bool     `json:"hidden" query:"type:equal,field:hidden,omitempty"`
}

func (d *DB) GetMenuById(mid int, preload bool) (*SysMenu, error) {
	var menu SysMenu
	ctx := d.orm.Model(&SysMenu{}).
		Where("id = ?", mid)
	if preload {
		ctx = Preload(ctx).
			Order("sort asc")
	}
	err := ctx.First(&menu).Error
	if err != nil {
		return nil, err
	}
	return &menu, nil
}

func (d *DB) GetMenuByName(com string) (*SysMenu, error) {
	var menu SysMenu
	err := d.orm.Model(&SysMenu{}).
		Where("name = ?", com).
		First(&menu).Error
	if err != nil {
		return nil, err
	}
	return &menu, nil
}

func (d *DB) GetMenuByCom(com string) (*SysMenu, error) {
	var menu SysMenu
	err := d.orm.Model(&SysMenu{}).
		Where("com = ?", com).
		First(&menu).Error
	if err != nil {
		return nil, err
	}
	return &menu, nil
}

func (d *DB) GetMenuByComName(comName string) (*SysMenu, error) {
	var menu SysMenu
	err := d.orm.Model(&SysMenu{}).
		Where("com_name = ?", comName).
		First(&menu).Error
	if err != nil {
		return nil, err
	}
	return &menu, nil
}

func (d *DB) GetAllMenus() ([]SysMenu, error) {
	var menus []SysMenu
	err := d.orm.Model(&SysMenu{}).
		Order("sort asc").
		Find(&menus).Error
	if err != nil {
		return nil, err
	}
	return menus, nil
}

func (d *DB) QueryMenus(filter MenuFilter) ([]SysMenu, error) {
	var menus []SysMenu
	query := d.orm.Model(&SysMenu{})
	query = BuildWhere(query, filter)
	err := query.Find(&menus).
		Order("sort asc").Error
	if err != nil {
		return nil, err
	}
	return menus, nil
}

func (d *DB) GetMenuTree(pid int) ([]SysMenu, error) {
	var menus []SysMenu
	ctx := d.orm.Model(&SysMenu{}).
		Where("pid = ?", pid)
	ctx = Preload(ctx)
	err := ctx.Order("sort asc").
		Find(&menus).Error
	if err != nil {
		return nil, err
	}
	return menus, nil
}

func (d *DB) GetMenuStatusByPid(pid int) ([]bool, error) {
	var status []bool
	err := d.orm.Model(&SysMenu{}).Select("hidden").
		Where("pid = ?", pid).Find(&status).Error
	if err != nil {
		return nil, err
	}
	return status, nil
}

func (d *DB) CreateMenu(m *SysMenu) (*SysMenu, error) {
	err := d.orm.Model(&SysMenu{}).Create(m).Error
	if err != nil {
		return nil, err
	}
	return d.GetMenuById(m.Id, true)
}

func (d *DB) UpdateMenu(m *SysMenu) (*SysMenu, error) {
	err := d.orm.Model(&SysMenu{}).Where("id = ?", m.Id).Save(m).Error
	if err != nil {
		return nil, err
	}
	return d.GetMenuById(m.Id, true)
}

func (d *DB) DeleteMenuById(menu *SysMenu) error {
	tx := d.orm.Begin()
	if len(menu.Children) > 0 {
		children := getMenuChilds(menu)
		var ids []int
		for _, child := range children {
			errUntied := tx.Table("sys_role_menu").Where("sys_menu_id = ?", child.Id).Delete(&SysRoleMenu{}).Error
			if errUntied != nil {
				tx.Rollback()
				return errUntied
			}
			ids = append(ids, child.Id)
		}
		errDelChild := tx.Model(&SysMenu{}).Where("id in (?)", ids).Delete(&SysMenu{}).Error
		if errDelChild != nil {
			tx.Rollback()
			return errDelChild
		}
	}
	errFather := tx.Table("sys_role_menu").Where("sys_menu_id = ?", menu.Id).Delete(&SysRoleMenu{}).Error
	if errFather != nil {
		tx.Rollback()
		return errFather
	}
	err := tx.Model(&SysMenu{}).Where("id = ?", menu.Id).Delete(&SysMenu{}).Error
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func getMenuChilds(menu *SysMenu) []SysMenu {
	var menus []SysMenu
	for _, child := range menu.Children {
		menus = append(menus, child)
		if len(menu.Children) > 0 {
			menus = append(menus, getMenuChilds(&child)...)
		}
	}
	return menus
}
