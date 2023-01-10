package db

import "time"

type SysPermission struct {
	Id          int       `json:"id" gorm:"type:int not null;autoIncrement;primaryKey;unique;uniqueIndex"`
	Name        string    `json:"name" gorm:"type:varchar(50)"`
	Description string    `json:"description" gorm:"type:varchar(45)"`
	Scope       string    `json:"scope" gorm:"type:varchar(20)"`
	Bid         int       `json:"bid" gorm:"type:int not null"`
	Ct          time.Time `json:"ct" gorm:"type:datetime not null;default:CURRENT_TIMESTAMP"`
	Menu        SysMenu   `json:"menu" gorm:"foreignKey:bid;constraint:OnUpdate:NoAction,OnDelete:NoAction;default:null"`
}

type PermitFilter struct {
	Blurry string `json:"blurry" query:"type:in_like,field:name|description,omitempty"`
}

func (d *DB) GetPermissions(preload bool) ([]SysPermission, error) {
	var permits []SysPermission
	ctx := d.orm.Model(&SysPermission{})
	if preload {
		ctx = ctx.Preload("Menu")
	}
	err := ctx.Find(&permits).Order("bid asc").Error
	if err != nil {
		return nil, err
	}
	return permits, nil
}

func (d *DB) QueryPermission(filter PermitFilter, preload bool) ([]SysPermission, error) {
	var permits []SysPermission
	ctx := d.orm.Model(&SysPermission{})
	ctx = BuildWhere(ctx, filter)
	if preload {
		ctx = ctx.Preload("Menu")
	}
	err := ctx.Find(&permits).Order("bid asc").Error
	if err != nil {
		return nil, err
	}
	return permits, nil
}

func (d *DB) GetPermissionById(pid int, preload bool) (*SysPermission, error) {
	var permit SysPermission
	ctx := d.orm.Model(&SysPermission{}).
		Where("id = ?", pid)
	if preload {
		ctx = ctx.Preload("Menu")
	}
	err := ctx.First(&permit).Error
	if err != nil {
		return nil, err
	}
	return &permit, nil
}

func (d *DB) GetPermissionByName(name string, preload bool) (*SysPermission, error) {
	var permit SysPermission
	ctx := d.orm.Model(&SysPermission{}).
		Where("name = ?", name)
	if preload {
		ctx = ctx.Preload("Menu")
	}
	err := ctx.First(&permit).Error
	if err != nil {
		return nil, err
	}
	return &permit, nil
}

func (d *DB) CreatePermission(permit *SysPermission) (*SysPermission, error) {
	res := d.orm.Model(&SysPermission{}).Create(permit)
	if res.Error != nil {
		return nil, res.Error
	}
	return d.GetPermissionById(permit.Id, true)
}

func (d *DB) UpdatePermit(permit *SysPermission) (*SysPermission, error) {
	err := d.orm.Model(&SysPermission{}).Where("id = ?", permit.Id).Save(permit).Error
	if err != nil {
		return nil, err
	}
	return d.GetPermissionById(permit.Id, true)
}

func (d *DB) DeletePermitById(permit *SysPermission) error {
	tx := d.orm.Begin()
	err := tx.Table("sys_role_permission").
		Where("sys_permission_id = ?", permit.Id).
		Delete(&SysRolePermission{}).
		Error
	if err != nil {
		tx.Rollback()
		return err
	}
	err = tx.Model(&SysPermission{}).
		Where("id = ?", permit.Id).
		Delete(&SysPermission{}).
		Error
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}
