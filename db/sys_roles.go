package db

import (
	"time"
)

type SysRole struct {
	Id          int             `json:"id" gorm:"type:int not null;autoIncrement;primaryKey;unique;uniqueIndex"`
	Code        string          `json:"code" gorm:"type:varchar(20)"`
	Name        string          `json:"name" gorm:"type:varchar(45)"`
	Level       int             `json:"level" gorm:"type:int not null"`
	Ct          time.Time       `json:"ct" gorm:"type:datetime not null;default:CURRENT_TIMESTAMP"`
	Menus       []SysMenu       `json:"menus" gorm:"many2many:sys_role_menu"`
	Permissions []SysPermission `json:"permissions" gorm:"many2many:sys_role_permission"`
}

type SysRoleMenu struct {
	SysMenuId int `json:"sys_menu_id" gorm:"type:int not null"`
	SysRoleId int `json:"sys_role_id" gorm:"type:int not null"`
}
type SysRolePermission struct {
	SysPermissionId int `json:"sys_permission_id" gorm:"type:int not null"`
	SysRoleId       int `json:"sys_role_id" gorm:"type:int not null"`
}

type SysUserRole struct {
	SysUserId int `json:"sys_user_id" gorm:"type:varchar(21) not null"`
	SysRoleId int `json:"sys_role_id" gorm:"type:int not null"`
}

type RoleFilter struct {
	Blurry string `json:"blurry" query:"type:in_like,field:code|name,omitempty"`
}

func (d *DB) GetRoles(preload bool) ([]SysRole, error) {
	var roles []SysRole
	ctx := d.orm.Model(&SysRole{})
	if preload {
		ctx = Preload(ctx)
	}
	err := ctx.Find(&roles).Order("level asc").Error
	if err != nil {
		return nil, err
	}
	return roles, nil
}

func (d *DB) QueryRoles(filter RoleFilter, preload bool) ([]SysRole, error) {
	var roles []SysRole
	ctx := d.orm.Model(&SysRole{})
	ctx = BuildWhere(ctx, filter)
	if preload {
		ctx = Preload(ctx)
	}
	err := ctx.Find(&roles).Order("level asc").Error
	if err != nil {
		return nil, err
	}
	return roles, nil
}

func (d *DB) GetRoleById(id int, preload bool) (*SysRole, error) {
	var role SysRole
	ctx := d.orm.Model(&SysRole{}).
		Where("id = ?", id)
	if preload {
		ctx = Preload(ctx)
	}
	err := ctx.First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (d *DB) GetRoleByName(name string) (*SysRole, error) {
	var role SysRole
	err := d.orm.Model(&SysRole{}).
		Where("name = ?", name).
		First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (d *DB) GetRoleByCode(code string) (*SysRole, error) {
	var role SysRole
	err := d.orm.Model(&SysRole{}).
		Where("code = ?", code).
		First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (d *DB) CreateRole(role *SysRole) (*SysRole, error) {
	err := d.orm.Model(&SysRole{}).Create(role).Error
	if err != nil {
		return nil, err
	}
	return d.GetRoleById(role.Id, true)
}

func (d *DB) UpdateRole(role *SysRole) (*SysRole, error) {
	err := d.orm.Model(&SysRole{}).Where("id = ?", role.Id).Save(role).Error
	if err != nil {
		return nil, err
	}
	return d.GetRoleById(role.Id, true)
}

func (d *DB) UpdateRoleRelations(role *SysRole, relation string, aso any) (*SysRole, error) {
	ctx := d.orm.Model(role)
	err := ctx.Association(relation).Replace(aso)
	if err != nil {
		return nil, err
	}
	return d.GetRoleById(role.Id, true)
}

func (d *DB) DeleteRoleById(rid int) error {
	role, err := d.GetRoleById(rid, false)
	if err != nil {
		return err
	}
	tx := d.orm.Begin()
	// down direct relations clear
	err = tx.Model(role).Association("Menus").Clear()
	if err != nil {
		tx.Rollback()
		return err
	}
	err = tx.Model(role).Association("Permissions").Clear()
	if err != nil {
		tx.Rollback()
		return err
	}
	// up direct relations clear
	err = tx.Table("sys_user_role").Where("sys_role_id = ?", rid).Delete(&SysUserRole{}).Error
	if err != nil {
		tx.Rollback()
		return err
	}
	err = tx.Where("id = ?", role.Id).Delete(role).Error
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}
