package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang-ast/conf"
	"golang-ast/db"
	"golang-ast/infra"
	"golang-ast/server"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "storm-admin-server",
	Short: "Storm Admin Server",
	Long:  `Storm Admin Server.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		confFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return
		}
		start(confFile)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "conf/conf.yml", "config file (default is conf/conf.yml)")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.Getwd()
		cobra.CheckErr(err)
		// Search config in home directory with name ".gin-test" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName("conf")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		_, _ = fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

var app *server.AdminServer

func start(confFile string) {
	confIns, err := conf.InitConf(confFile, func(config interface{}) {
	})
	if err != nil {
		fmt.Printf("init conf failed, err:%v\n", err)
		return
	}
	logger := infra.InitLogger(confIns.LogCfg)
	dbms, err := db.Init(confIns, logger)
	if err != nil {
		logger.Error("db init failed, err", zap.Error(err))
		return
	}
	app, err = server.NewServer(confIns, logger, dbms)
	if err != nil {
		logger.Error("init app failed, err", zap.Error(err))
		return
	}
	go app.StartHttpServer()
	logger.Info("\u001B[32m start storm admin server complete\u001B[0m")
	handleProcessSignal(logger)
}

var signChan = make(chan os.Signal)

func handleProcessSignal(log *zap.Logger) {
	var sig os.Signal
	signal.Notify(
		signChan,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGTERM,
		syscall.SIGABRT,
	)
	for {
		sig = <-signChan
		log.Sugar().Infof(`signal received: %s`, sig.String())
		switch sig {
		// Shutdown the servers.
		case syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGTERM, syscall.SIGKILL:
			app.Close()
			return
		default:
		}
	}
}

func main() {
	err := os.Setenv("TZ", "UTC")
	if err != nil {
		return
	}
	debug.SetMaxStack(4 * 1024 * 1024 * 1024)
	runtime.GOMAXPROCS(runtime.NumCPU() * 4)
	Execute()
}
