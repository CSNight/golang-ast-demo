package infra

import (
	"errors"
	"golang-ast/conf"
	"golang-ast/db"
	"golang-ast/utils"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type CtxKey string

var authHandler *Authorization

func GetAuthHandler() *Authorization {
	return authHandler
}

type Authentication struct {
	authorities     *hashset.Set
	principal       string
	credentials     string
	isAuthenticated bool
}

func (a *Authentication) Authorities() *hashset.Set {
	return a.authorities
}

func (a *Authentication) SetAuthorities(authorities *hashset.Set) {
	a.authorities = authorities
}

func (a *Authentication) Principal() string {
	return a.principal
}

func (a *Authentication) Credentials() string {
	return a.credentials
}

func (a *Authentication) IsAuthenticated() bool {
	return a.isAuthenticated
}

func (a *Authentication) SetIsAuthenticated(isAuthenticated bool) {
	a.isAuthenticated = isAuthenticated
}

type FailStatus struct {
	times int
	ut    time.Time
}

type Authorization struct {
	jwt           *JWT
	cfg           *conf.AuthConfig
	log           *zap.Logger
	db            *db.DB
	lock          *sync.RWMutex
	trie          *Trie
	tokenStore    sync.Map
	monitor       *time.Ticker
	quit          chan bool
	authorizes    map[string]*Authentication
	loginList     *hashset.Set
	lockList      map[string]time.Time
	loginFailList map[string]FailStatus
}

func NewAuthorization(cfg *conf.AuthConfig, db *db.DB, log *zap.Logger) {
	authHandler = &Authorization{
		jwt:           NewJWT(cfg),
		db:            db,
		log:           log,
		cfg:           cfg,
		tokenStore:    sync.Map{},
		lock:          &sync.RWMutex{},
		trie:          NewTrie(),
		loginList:     hashset.New(),
		lockList:      map[string]time.Time{},
		loginFailList: map[string]FailStatus{},
		authorizes:    make(map[string]*Authentication),
		monitor:       time.NewTicker(time.Minute),
		quit:          make(chan bool, 1),
	}
	for _, v := range cfg.Permits.Authentications {
		authHandler.trie.Parse("/api"+v.Url, strings.Split(v.Permit, "|")[0])
	}
	for _, k := range cfg.Permits.WhiteList {
		authHandler.trie.Parse("/api"+k, "*")
	}
	go authHandler.monitorTick()
}

func (a *Authorization) Close() {
	a.quit <- true
}

func (a *Authorization) IsAlreadyLogin(username string) bool {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.loginList.Contains(username)
}

func (a *Authorization) CreateToken(jwtId string, user *db.SysUser) (string, error) {
	var roles []string
	for _, role := range user.Roles {
		roles = append(roles, role.Code)
	}
	return a.jwt.CreateToken(jwtId, user.Id, user.Name, roles)
}

func (a *Authorization) ParseToken(token string) (*JWTClaims, error) {
	return a.jwt.ParseToken(token)
}

func (a *Authorization) TrieSearch(path string) (bool, any, map[string]string) {
	match, err := a.trie.Match(path)
	if err != nil || match == nil {
		return false, nil, nil
	}
	if match.Node == nil {
		return false, nil, nil
	}
	return true, match.Node.Value, match.Params
}

func (a *Authorization) RefreshToken(token string) (string, error) {
	return a.jwt.RefreshToken(token)
}

func (a *Authorization) OnAuthSuccessHandler(user *db.SysUser, ctx *fiber.Ctx) error {
	if tokenObj, ok := a.tokenStore.Load(user.Id); ok {
		_, err := a.ParseToken(tokenObj.(string))
		if err == nil {
			return OkWithMessage(map[string]string{
				"username": user.Name,
				"tk":       tokenObj.(string),
				"status":   "Login success",
			}, ctx)
		}
	}
	jwtId := utils.MustNanoId()
	token, err := a.CreateToken(jwtId, user)
	if err != nil {
		return FailWithMessage(http.StatusForbidden, "token gen failed:"+err.Error(), ctx)
	}
	a.tokenStore.Store(jwtId, token)
	a.lock.Lock()
	a.authorizes[jwtId] = &Authentication{
		principal:       user.Name,
		credentials:     user.Password,
		isAuthenticated: true,
		authorities:     createAuthorities(user.Roles),
	}
	a.loginList.Add(user.Name)
	a.lock.Unlock()
	a.SetAuthentication(ctx, jwtId)
	go func() {
		_, err = a.db.UpdateUserWithId(user, map[string]any{
			"id":          user.Id,
			"login_times": user.LoginTimes + 1,
		}, nil)
		if err != nil {
			a.log.Error("update login times error", zap.Error(err))
		}
	}()
	if user.GrantBy == "github" {
		return ctx.Redirect(a.cfg.RedirectUrl + "?grant=git&tk=" + token)
	}
	return OkWithMessage(map[string]string{
		"username": user.Name,
		"tk":       token,
		"status":   "Login success",
	}, ctx)
}

func (a *Authorization) OnAuthFailedHandler(identify string, err error, ctx *fiber.Ctx) error {
	a.lock.RLock()
	defer a.lock.RUnlock()
	if totalFails, ok := a.loginFailList[identify]; ok {
		totalFails.times = totalFails.times + 1
		totalFails.ut = time.Now()
		a.loginFailList[identify] = totalFails
		if totalFails.times >= 5 {
			user, _ := a.db.FindUserByIdentify(identify, false)
			if user != nil {
				_, errLock := a.db.UpdateUserWithId(user, map[string]any{
					"id":      user.Id,
					"enable":  false,
					"lock_by": "lockByFails",
				}, nil)
				if errLock != nil {
					a.log.Error("OnAuthFailedHandler().UpdateUser(). update user lock error", zap.Error(errLock))
					return errLock
				}
				delete(a.loginFailList, identify)
				a.lockList[identify] = time.Now().Add(time.Second * 60)
				go a.unlockJob(identify)
				err = errors.New("错误次数过多，账户已锁定，解锁时间60秒后")
			}
		}
	} else {
		a.loginFailList[identify] = FailStatus{
			times: 1,
			ut:    time.Now(),
		}
	}
	if exp, ok := a.lockList[identify]; ok && identify != "" {
		remain := int(time.Until(exp).Seconds())
		err = errors.New("错误次数过多，账户已锁定，" + strconv.Itoa(remain) + "秒后解锁")
		if remain == 0 {
			err = errors.New("账户锁定已解除")
		}
	}
	return FailWithMessage(http.StatusUnauthorized, err, ctx)
}

func (a *Authorization) OnSignOutHandler(claim *JWTClaims, ctx *fiber.Ctx) error {
	a.lock.Lock()
	defer a.lock.Unlock()
	delete(a.authorizes, claim.ID)
	a.loginList.Remove(claim.Name)
	a.tokenStore.Delete(claim.ID)
	return OkWithMessage(claim.Name, ctx)
}

func (a *Authorization) SetAuthentication(ctx *fiber.Ctx, id string) {
	a.lock.RLock()
	defer a.lock.RUnlock()
	ctx.Locals("auth", a.authorizes[id])
}

func (a *Authorization) GetAuthentication(claim *JWTClaims) (*Authentication, error) {
	a.lock.RLock()
	permit, ok := a.authorizes[claim.ID]
	a.lock.RUnlock()
	if ok {
		return permit, nil
	} else {
		u, err := a.db.FindUserByIdentify(claim.Name, true)
		if err != nil {
			a.log.Error("GetAuthentication().loadUserByName error", zap.Error(err))
			return nil, err
		}
		if !u.Enable || u.LockBy != "none" {
			a.log.Error("GetAuthentication().Try load disabled or locked user")
			return nil, errors.New("user has been disabled")
		}
		jwtId := utils.MustNanoId()
		permit = &Authentication{
			principal:       u.Name,
			credentials:     u.Password,
			isAuthenticated: true,
			authorities:     createAuthorities(u.Roles),
		}
		a.lock.Lock()
		a.authorizes[jwtId] = permit
		a.lock.Unlock()
		return permit, nil
	}
}

func (a *Authorization) RemoveAuthentication(username string) {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.loginList.Contains(username) {
		a.loginList.Remove(username)
	}
	for k, v := range a.authorizes {
		if v.principal == username {
			delete(a.authorizes, k)
			a.tokenStore.Delete(k)
			break
		}
	}
}

func (a *Authorization) monitorTick() {
	defer a.monitor.Stop()
	for {
		select {
		case <-a.quit:
			close(a.quit)
			return
		case <-a.monitor.C:
			a.cleanList()
		}
	}
}

func (a *Authorization) cleanList() {
	a.lock.Lock()
	defer a.lock.Unlock()
	for k, v := range a.loginFailList {
		if _, ok := a.lockList[k]; !ok && time.Since(v.ut).Seconds() > 100 {
			delete(a.loginFailList, k)
		}
	}
}

func (a *Authorization) unlockJob(identify string) {
	time.Sleep(time.Second * 60)
	user, _ := a.db.FindUserByIdentify(identify, false)
	if user != nil {
		_, err := a.db.UpdateUserWithId(user, map[string]any{
			"id":      user.Id,
			"enable":  true,
			"lock_by": "none",
		}, nil)
		if err != nil {
			a.log.Error("unlockJob().UpdateUser(). update user lock error", zap.Error(err))
		}
	}
	a.lock.Lock()
	defer a.lock.Unlock()
	delete(a.lockList, identify)
}

func createAuthorities(roles []db.SysRole) *hashset.Set {
	auths := hashset.New()
	for _, role := range roles {
		for _, permit := range role.Permissions {
			auths.Add(permit.Name)
		}
	}
	return auths
}
