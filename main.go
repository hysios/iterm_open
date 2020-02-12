package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	sysexec "os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var (
	logger  *log.Logger
	logfile *os.File
)

func init() {
	viper.SetDefault("open.default", "/usr/bin/open")
	viper.SetDefault("open.editor", "/usr/local/bin/code-insiders")
	viper.SetDefault("logger_file", "/tmp/iterm_open.log")

	viper.SetConfigName(".iterm_open") // name of config file (without extension)
	// Find home directory.
	home, err := homedir.Dir()
	if err != nil {
		fmt.Print(err)
	}

	// Search config in home directory with name ".cobra" (without extension).
	viper.AddConfigPath(filepath.Join(home, "config"))
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
		} else {
			// Config file was found but another error was produced
		}
	}

}

func main() {
	flag.Parse()
	var err error
	logname := viper.GetString("logger_file")
	// logfile, err = os.Open(logname)
	logfile, err := os.OpenFile(logname, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.Printf("can't open logger file %s", logname)
		return
	}

	logger = log.New(logfile, "itermopen", log.Lshortfile|log.LstdFlags)
	defer logfile.Close()

	var (
		pwd     string
		file    string
		orgfile string
		lineno  string
		colno   string
	)

	var parseLine = func(fileline string) {
		ss := strings.SplitN(fileline, ":", 2)
		if len(ss) == 2 {
			file = ss[0]
			lineno = ss[1]
		} else {
			file = ss[0]
		}
	}

	var parseLinecol = func(linecol string) {
		ss := strings.SplitN(linecol, ":", 2)
		if len(ss) >= 2 {
			lineno = ss[0]
			colno = strings.TrimRight(ss[1], ":")
		}
	}

	args := flag.Args()
	logger.Printf("args: %v\n", args)
	if len(args) >= 3 {
		pwd = args[0]
		file = args[1]
		orgfile = file
		lineno = args[2]
	} else if len(args) >= 2 {
		pwd = args[0]
		file = args[1]
		orgfile = file

		parseLine(file)

	} else if len(args) >= 1 {
		pwd, file = filepath.Dir(args[0]), filepath.Base(args[0])
		orgfile = file
		parseLine(file)
	} else {
		log.Fatal("must have args")
	}

	if len(lineno) > 0 {
		parseLinecol(lineno)
	}

	switch {
	case isURI(orgfile):
		exec(viper.GetString("open.default"), orgfile)
	case len(lineno) > 0:
		var (
			filename string
			ok       bool
		)
		logger.Printf("file: %v lineno: %s colno: %s\n", file, lineno, colno)
		if filepath.IsAbs(file) {
			filename = file
		} else {
			filename, ok = lookupFile(pwd, file)
			if !ok {
				return
			}
		}

		var openarg string
		if len(colno) > 0 {
			openarg = fmt.Sprintf("%s:%s:%s", filename, lineno, colno)
		} else {
			openarg = fmt.Sprintf("%s:%s", filename, lineno)
		}
		logger.Printf("%s %s\n", viper.GetString("open.editor"), openarg)
		exec(viper.GetString("open.editor"), "-r", "-g", openarg)
	default:

		exec(viper.GetString("open.default"), file)
	}

	logger.Printf("args: %v pwd: %s file: %s lineno: %s colno: %s\n", args, pwd, file, lineno, colno)
}

var ErrFound = errors.New("found")

var reURI = regexp.MustCompile(`http(s)?://[\w.-]+`)

func isURI(filename string) bool {
	return reURI.MatchString(filename)
}

func lookupFile(dir string, file string) (result string, ok bool) {
	log.Printf("absoluate file %s\n", filepath.Join(dir, file))

	if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
		return filepath.Join(dir, file), true
	} else if os.IsExist(err) {
		return filepath.Join(dir, file), true
	}

	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, file) && !info.IsDir() {
			result = filepath.Join(result, path)
			ok = true
			return ErrFound
		}

		return nil
	}); err == ErrFound {
		return
	}

	return
}

func exec(cmd string, args ...string) error {
	fmt.Printf("cmd: %s %v\n", cmd, args)
	return sysexec.Command(cmd, args...).Run()
}
