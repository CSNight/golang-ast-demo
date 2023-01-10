package db

import (
	"golang-ast/utils"
	"math"
	"sort"
	"time"

	"gorm.io/gorm/clause"
)

type SysUser struct {
	Id          string    `json:"id" gorm:"type:varchar(21);primaryKey;unique;uniqueIndex"`
	Name        string    `json:"name" gorm:"type:varchar(50)"`
	Password    string    `json:"password" gorm:"type:varchar(200)"`
	Nick        string    `json:"nick" gorm:"type:varchar(50)"`
	Email       string    `json:"email" gorm:"type:varchar(50)"`
	Phone       string    `json:"phone" gorm:"type:varchar(20)"`
	Enable      bool      `json:"enable" gorm:"type:tinyint(1)"`
	GrantBy     string    `json:"grant_by" gorm:"type:varchar(10)"`
	LoginTimes  int       `json:"login_times" gorm:"type:int not null"`
	LockBy      string    `json:"lock_by" gorm:"type:varchar(10)"`
	Header      string    `json:"header" gorm:"type:varchar(300)"`
	Ct          time.Time `json:"ct" gorm:"type:datetime not null;default:CURRENT_TIMESTAMP"`
	Ut          time.Time `json:"ut" gorm:"type:datetime not null;default:CURRENT_TIMESTAMP"`
	Roles       []SysRole `json:"roles" gorm:"many2many:sys_user_role"`
	Authorities []string  `json:"authorities" gorm:"-:all"`
}

type UserFilter struct {
	Idx  []string `json:"idx" query:"type:in,field:id,omitempty"`
	Name string   `json:"name" query:"type:in_like,field:name|phone|email,omitempty"`
}

func (d *DB) CreateUser(u *SysUser) (*SysUser, error) {
	u.Id = utils.MustNanoId()
	if u.Nick == "" {
		u.Nick = u.Name
	}
	err := d.orm.Model(&SysUser{}).Create(u).Error
	if err != nil {
		return nil, err
	}
	return d.FindUserById(u.Id, false)
}

func (d *DB) QueryUsers(filter UserFilter, size int32, off int32, order string) (*Page, error) {
	var users []SysUser
	query := d.orm.Model(&SysUser{})
	query = BuildWhere(query, filter).Omit("password")
	if order != "" {
		query = query.Order(order)
	}
	var count int64
	err := query.Count(&count).Error
	if err != nil {
		return nil, err
	}
	if count > 0 {
		if size > 0 {
			query = query.Limit(int(size)).Offset(int(off * size))
		}
		err = query.Preload("Roles").Find(&users).Error
		if err != nil {
			return nil, err
		}
	}
	var pages int64 = 0
	if count == 0 || size == 0 {
		pages = 1
	} else {
		pages = int64(math.Ceil(float64(count) / float64(size)))
	}
	userSort(users)
	page := Page{
		TotalPages:    pages,
		TotalElements: count,
		Content:       users,
	}
	return &page, nil
}

func (d *DB) FindAllUsers() ([]SysUser, error) {
	var users []SysUser
	err := d.orm.Model(&SysUser{}).Omit("password").Preload("Roles").Find(&users).Error
	if err != nil {
		return nil, err
	}
	userSort(users)

	return users, nil
}

func (d *DB) FindAllEnabledUsers() ([]SysUser, error) {
	var users []SysUser
	err := d.orm.Model(&SysUser{}).Where("enable = ?", true).Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (d *DB) FindUserById(uid string, preload bool) (*SysUser, error) {
	var u SysUser
	ctx := d.orm.Model(&SysUser{})
	if preload {
		ctx = ctx.Preload(clause.Associations, Preload)
	}
	err := ctx.Where("id = ?", uid).First(&u).Error
	if err != nil {
		return nil, err
	}
	if preload {
		u.Authorities = fillAuthorities(u.Roles)
	}
	return &u, nil
}

func (d *DB) FindUserByIdentify(identify string, preload bool) (*SysUser, error) {
	var u SysUser
	ctx := d.orm.Model(&SysUser{}).
		Or("name = ?", identify).
		Or("email = ?", identify).
		Or("phone = ?", identify)
	if preload {
		ctx = ctx.Preload(clause.Associations, Preload)
	}
	err := ctx.First(&u).Error
	if err != nil {
		return nil, err
	}
	if preload {
		u.Authorities = fillAuthorities(u.Roles)
	}
	return &u, nil
}

func (d *DB) QueryUserBy(name, email, phone string) (*SysUser, error) {
	var u SysUser
	ctx := d.orm.Model(&SysUser{})
	if name != "" {
		ctx = ctx.Or("name = ?", name)
	}
	if email != "" {
		ctx = ctx.Or("email = ?", name)
	}
	if phone != "" {
		ctx = ctx.Or("phone = ?", name)
	}
	err := ctx.First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (d *DB) UpdateUserIgnoreEmpty(u *SysUser) (*SysUser, error) {
	var ux SysUser
	u.Ut = time.Now()
	ctx := d.orm.Model(u)
	err := ctx.Updates(u).Association("Roles").Replace(u.Roles)
	if err != nil {
		return nil, err
	}
	err = ctx.First(&ux).Error
	if err != nil {
		return nil, err
	}
	return &ux, nil
}

func (d *DB) UpdateUserWithId(u *SysUser, m map[string]any, aso any) (*SysUser, error) {
	m["ut"] = time.Now()
	var ux SysUser
	// common field update by where clause
	ctx := d.orm.Model(&SysUser{}).Where("id = ?", m["id"])
	var err error
	if aso != nil {
		err = ctx.Updates(m).Error
		if err != nil {
			return nil, err
		}
		// relation replace with isolated ctx
		err = d.orm.Model(u).Association("Roles").Replace(aso)
	} else {
		err = ctx.Updates(m).Error
	}
	if err != nil {
		return nil, err
	}
	err = ctx.First(&ux).Error
	if err != nil {
		return nil, err
	}
	return &ux, nil
}

func (d *DB) DeleteUserById(uid string) (string, error) {
	u, err := d.FindUserById(uid, false)
	if err != nil {
		return "", err
	}
	tx := d.orm.Begin()
	err = tx.Model(u).Association("Roles").Clear()
	if err != nil {
		tx.Rollback()
		return "", err
	}
	err = tx.Where("id = ?", u.Id).Delete(u).Error
	if err != nil {
		tx.Rollback()
		return "", err
	}
	return u.Name, tx.Commit().Error

}

func (d *DB) DeleteUserByName(name string) error {
	u, err := d.FindUserByIdentify(name, false)
	if err != nil {
		return err
	}
	tx := d.orm.Begin()
	err = tx.Model(u).Association("Roles").Clear()
	if err != nil {
		tx.Rollback()
		return err
	}
	err = tx.Where("id = ?", u.Id).Delete(u).Error
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func fillAuthorities(roles []SysRole) []string {
	permits := make([]string, 0)
	for _, role := range roles {
		for _, permit := range role.Permissions {
			permits = append(permits, permit.Name)
		}
	}
	return permits
}
func userSort(users []SysUser) {
	if users == nil {
		return
	}
	sort.SliceStable(users, func(i, j int) bool {
		t0 := 10
		t1 := 10
		for _, role := range users[i].Roles {
			if role.Level < t0 {
				t0 = role.Level
			}
		}
		for _, role := range users[j].Roles {
			if role.Level < t1 {
				t1 = role.Level
			}
		}
		return t0 < t1
	})
}
