package conf

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed permit.yml
var permitCfg []byte

type AppConfig struct {
	HttpAddr string `yaml:"http_addr" json:"http_addr"`
	DbConn   string `yaml:"db_conn" json:"db_conn"`
}

type AuthConfig struct {
	JwtKey      string        `yaml:"jwt_key" json:"jwt_key"`
	JwtExp      int           `yaml:"jwt_exp" json:"jwt_exp"`
	GitId       string        `yaml:"git_id" json:"git_id"`
	GitKey      string        `yaml:"git_key" json:"git_key"`
	RedirectUrl string        `yaml:"redirect_url" json:"redirect_url"`
	Permits     *PermitConfig `yaml:"permits" json:"permits"`
}
type AuthKV struct {
	Url    string `yaml:"url"`
	Permit string `yaml:"permit"`
}
type PermitConfig struct {
	Authentications []AuthKV `yaml:"permits"`
	WhiteList       []string `yaml:"white_list"`
}

type LogConfig struct {
	Level      string `yaml:"level" json:"level"`
	Filename   string `yaml:"filename" json:"filename"`
	MaxSize    int    `yaml:"max_size" json:"max_size"`
	MaxAge     int    `yaml:"max_age" json:"max_age"`
	MaxBackups int    `yaml:"max_backups" json:"max_backups"`
}

type GConfig struct {
	AppCfg  *AppConfig  `yaml:"app"`
	AuthCfg *AuthConfig `yaml:"auth"`
	LogCfg  *LogConfig  `yaml:"log"`
}

type Config struct {
	Environment          string
	ENVPrefix            string
	Debug                bool
	Verbose              bool
	Silent               bool
	AutoReload           bool
	AutoReloadInterval   time.Duration
	AutoReloadCallback   func(config interface{})
	ErrorOnUnmatchedKeys bool
}
type HotConfig struct {
	*Config
	configModTimes map[string]time.Time
}

// New initialize a HotConfig
func New(config *Config) *HotConfig {
	if config == nil {
		config = &Config{}
	}

	if os.Getenv("HotConfig_DEBUG_MODE") != "" {
		config.Debug = true
	}

	if os.Getenv("HotConfig_VERBOSE_MODE") != "" {
		config.Verbose = true
	}

	if os.Getenv("HotConfig_SILENT_MODE") != "" {
		config.Silent = true
	}

	if config.AutoReload && config.AutoReloadInterval == 0 {
		config.AutoReloadInterval = time.Second
	}

	return &HotConfig{Config: config}
}

var testRegexp = regexp.MustCompile(`_test|(\\.test$)`)

// GetEnvironment get environment
func (hc *HotConfig) GetEnvironment() string {
	if hc.Environment == "" {
		if env := os.Getenv("HotConfig_ENV"); env != "" {
			return env
		}

		if testRegexp.MatchString(os.Args[0]) {
			return "test"
		}

		return "development"
	}
	return hc.Environment
}

// GetErrorOnUnmatchedKeys returns a boolean indicating if an error should be
// thrown if there are keys in the config file that do not correspond to the
// config struct
func (hc *HotConfig) GetErrorOnUnmatchedKeys() bool {
	return hc.ErrorOnUnmatchedKeys
}

// Load will unmarshal configurations to struct from files that you provide
func (hc *HotConfig) Load(config interface{}, files ...string) (err error) {
	defaultValue := reflect.Indirect(reflect.ValueOf(config))
	if !defaultValue.CanAddr() {
		return fmt.Errorf("Config %v should be addressable", config)
	}
	err, _ = hc.load(config, false, files...)

	if hc.Config.AutoReload {
		go func() {
			timer := time.NewTimer(hc.Config.AutoReloadInterval)
			for range timer.C {
				reflectPtr := reflect.New(reflect.ValueOf(config).Elem().Type())
				reflectPtr.Elem().Set(defaultValue)

				var changed bool
				if err, changed = hc.load(reflectPtr.Interface(), true, files...); err == nil && changed {
					reflect.ValueOf(config).Elem().Set(reflectPtr.Elem())
					if hc.Config.AutoReloadCallback != nil {
						hc.Config.AutoReloadCallback(config)
					}
				} else if err != nil {
					fmt.Printf("Failed to reload configuration from %v, got error %v\n", files, err)
				}
				timer.Reset(hc.Config.AutoReloadInterval)
			}
		}()
	}
	return
}

func (hc *HotConfig) getENVPrefix() string {
	if hc.Config.ENVPrefix == "" {
		if prefix := os.Getenv("CONFIGOR_ENV_PREFIX"); prefix != "" {
			return prefix
		}
		return "HotConfig"
	}
	return hc.Config.ENVPrefix
}

func getConfigurationFileWithENVPrefix(file, env string) (string, time.Time, error) {
	var (
		envFile string
		extname = path.Ext(file)
	)

	if extname == "" {
		envFile = fmt.Sprintf("%v.%v", file, env)
	} else {
		envFile = fmt.Sprintf("%v.%v%v", strings.TrimSuffix(file, extname), env, extname)
	}

	if fileInfo, err := os.Stat(envFile); err == nil && fileInfo.Mode().IsRegular() {
		return envFile, fileInfo.ModTime(), nil
	}
	return "", time.Now(), fmt.Errorf("failed to find file %v", file)
}

func (hc *HotConfig) getConfigurationFiles(watchMode bool, files ...string) ([]string, map[string]time.Time) {
	var resultKeys []string
	var results = map[string]time.Time{}

	if !watchMode && (hc.Config.Debug || hc.Config.Verbose) {
		fmt.Printf("Current environment: '%v'\n", hc.GetEnvironment())
	}

	for i := len(files) - 1; i >= 0; i-- {
		foundFile := false
		file := files[i]

		// check configuration
		if fileInfo, err := os.Stat(file); err == nil && fileInfo.Mode().IsRegular() {
			foundFile = true
			resultKeys = append(resultKeys, file)
			results[file] = fileInfo.ModTime()
		}

		// check configuration with env
		if file, modTime, err := getConfigurationFileWithENVPrefix(file, hc.GetEnvironment()); err == nil {
			foundFile = true
			resultKeys = append(resultKeys, file)
			results[file] = modTime
		}

		// check example configuration
		if !foundFile {
			if example, modTime, err := getConfigurationFileWithENVPrefix(file, "example"); err == nil {
				if !watchMode && !hc.Silent {
					fmt.Printf("Failed to find configuration %v, using example file %v\n", file, example)
				}
				resultKeys = append(resultKeys, example)
				results[example] = modTime
			} else if !hc.Silent {
				fmt.Printf("Failed to find configuration %v\n", file)
			}
		}
	}
	return resultKeys, results
}

func processFile(config interface{}, file string, errorOnUnmatchedKeys bool) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if errorOnUnmatchedKeys {
		return yaml.Unmarshal(data, config)
	}
	return yaml.Unmarshal(data, config)
}

func getPrefixForStruct(prefixes []string, fieldStruct *reflect.StructField) []string {
	if fieldStruct.Anonymous && fieldStruct.Tag.Get("anonymous") == "true" {
		return prefixes
	}
	return append(prefixes, fieldStruct.Name)
}

func (hc *HotConfig) processDefaults(config interface{}) error {
	configValue := reflect.Indirect(reflect.ValueOf(config))
	if configValue.Kind() != reflect.Struct {
		return errors.New("invalid config, should be struct")
	}

	configType := configValue.Type()
	for i := 0; i < configType.NumField(); i++ {
		var (
			fieldStruct = configType.Field(i)
			field       = configValue.Field(i)
		)

		if !field.CanAddr() || !field.CanInterface() {
			continue
		}

		if isBlank := reflect.DeepEqual(field.Interface(), reflect.Zero(field.Type()).Interface()); isBlank {
			// Set default configuration if blank
			if value := fieldStruct.Tag.Get("default"); value != "" {
				if err := yaml.Unmarshal([]byte(value), field.Addr().Interface()); err != nil {
					return err
				}
			}
		}

		for field.Kind() == reflect.Ptr {
			field = field.Elem()
		}

		switch field.Kind() {
		case reflect.Struct:
			if err := hc.processDefaults(field.Addr().Interface()); err != nil {
				return err
			}
		case reflect.Slice:
			for i := 0; i < field.Len(); i++ {
				if reflect.Indirect(field.Index(i)).Kind() == reflect.Struct {
					if err := hc.processDefaults(field.Index(i).Addr().Interface()); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (hc *HotConfig) processTags(config interface{}, prefixes ...string) error {
	configValue := reflect.Indirect(reflect.ValueOf(config))
	if configValue.Kind() != reflect.Struct {
		return errors.New("invalid config, should be struct")
	}

	configType := configValue.Type()
	for i := 0; i < configType.NumField(); i++ {
		var (
			envNames    []string
			fieldStruct = configType.Field(i)
			field       = configValue.Field(i)
			envName     = fieldStruct.Tag.Get("env") // read configuration from shell env
		)

		if !field.CanAddr() || !field.CanInterface() {
			continue
		}

		if envName == "" {
			envNames = append(envNames, strings.Join(append(prefixes, fieldStruct.Name), "_"))                  // HotConfig_DB_Name
			envNames = append(envNames, strings.ToUpper(strings.Join(append(prefixes, fieldStruct.Name), "_"))) // CONFIGOR_DB_NAME
		} else {
			envNames = []string{envName}
		}

		if hc.Config.Verbose {
			fmt.Printf("Trying to load struct `%v`'s field `%v` from env %v\n", configType.Name(), fieldStruct.Name, strings.Join(envNames, ", "))
		}

		// Load From Shell ENV
		for _, env := range envNames {
			if value := os.Getenv(env); value != "" {
				if hc.Config.Debug || hc.Config.Verbose {
					fmt.Printf("Loading configuration for struct `%v`'s field `%v` from env %v...\n", configType.Name(), fieldStruct.Name, env)
				}

				switch reflect.Indirect(field).Kind() {
				case reflect.Bool:
					switch strings.ToLower(value) {
					case "", "0", "f", "false":
						field.Set(reflect.ValueOf(false))
					default:
						field.Set(reflect.ValueOf(true))
					}
				case reflect.String:
					field.Set(reflect.ValueOf(value))
				default:
					if err := yaml.Unmarshal([]byte(value), field.Addr().Interface()); err != nil {
						return err
					}
				}
				break
			}
		}

		if isBlank := reflect.DeepEqual(field.Interface(), reflect.Zero(field.Type()).Interface()); isBlank && fieldStruct.Tag.Get("required") == "true" {
			// return error if it is required but blank
			return errors.New(fieldStruct.Name + " is required, but blank")
		}

		for field.Kind() == reflect.Ptr {
			field = field.Elem()
		}

		if field.Kind() == reflect.Struct {
			if err := hc.processTags(field.Addr().Interface(), getPrefixForStruct(prefixes, &fieldStruct)...); err != nil {
				return err
			}
		}

		if field.Kind() == reflect.Slice {
			if arrLen := field.Len(); arrLen > 0 {
				for i := 0; i < arrLen; i++ {
					if reflect.Indirect(field.Index(i)).Kind() == reflect.Struct {
						if err := hc.processTags(field.Index(i).Addr().Interface(), append(getPrefixForStruct(prefixes, &fieldStruct), fmt.Sprint(i))...); err != nil {
							return err
						}
					}
				}
			} else {
				defer func(field reflect.Value, fieldStruct reflect.StructField) {
					if !configValue.IsZero() {
						// load slice from env
						newVal := reflect.New(field.Type().Elem()).Elem()
						if newVal.Kind() == reflect.Struct {
							idx := 0
							for {
								newVal = reflect.New(field.Type().Elem()).Elem()
								if err := hc.processTags(newVal.Addr().Interface(), append(getPrefixForStruct(prefixes, &fieldStruct), fmt.Sprint(idx))...); err != nil {
									return // err
								} else if reflect.DeepEqual(newVal.Interface(), reflect.New(field.Type().Elem()).Elem().Interface()) {
									break
								} else {
									idx++
									field.Set(reflect.Append(field, newVal))
								}
							}
						}
					}
				}(field, fieldStruct)
			}
		}
	}
	return nil
}

func (hc *HotConfig) load(config interface{}, watchMode bool, files ...string) (err error, changed bool) {
	defer func() {
		if hc.Config.Debug || hc.Config.Verbose {
			if err != nil {
				fmt.Printf("Failed to load configuration from %v, got %v\n", files, err)
			}

			fmt.Printf("Configuration:\n  %#v\n", config)
		}
	}()

	configFiles, configModTimeMap := hc.getConfigurationFiles(watchMode, files...)

	if watchMode {
		if len(configModTimeMap) == len(hc.configModTimes) {
			var changed bool
			for f, t := range configModTimeMap {
				if v, ok := hc.configModTimes[f]; !ok || t.After(v) {
					changed = true
				}
			}

			if !changed {
				return nil, false
			}
		}
	}

	// process defaults
	_ = hc.processDefaults(config)

	for _, file := range configFiles {
		if hc.Config.Debug || hc.Config.Verbose {
			fmt.Printf("Loading configurations from file '%v'...\n", file)
		}
		if err = processFile(config, file, hc.GetErrorOnUnmatchedKeys()); err != nil {
			return err, true
		}
	}
	hc.configModTimes = configModTimeMap

	if prefix := hc.getENVPrefix(); prefix == "-" {
		err = hc.processTags(config)
	} else {
		err = hc.processTags(config, prefix)
	}

	return err, true
}

func InitConf(path string, cb func(config interface{})) (*GConfig, error) {
	t := GConfig{}
	err := New(&Config{
		AutoReload:         true,
		AutoReloadInterval: time.Second * 5,
		AutoReloadCallback: func(config interface{}) {
			confStr, err := json.Marshal(config)
			if err != nil {
				return
			}
			fmt.Printf("new config %v \r\n", string(confStr))
			cb(config)
		},
	}).Load(&t, path)
	if err != nil {
		return nil, err
	}
	var permits PermitConfig
	err = yaml.Unmarshal(permitCfg, &permits)
	if err != nil {
		return nil, err
	}
	t.AuthCfg.Permits = &permits
	return &t, nil
}
