package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	vip "github.com/spf13/viper"
)

func Verify(v *vip.Viper) error {
	for _, varName := range v.AllKeys() {
		if varName == "admins" || varName == "discordtoken" {
			continue
		}
		value := v.GetString(varName)
		if value == "" {
			return fmt.Errorf("! %s unset. use --%s flag, personality config, or SOULSHACK_%s env", varName, varName, strings.ToUpper(varName))
		}

		if v.GetBool("verbose") {
			if varName == "openaikey" {
				value = strings.Repeat("*", len(value))
			}
			log.Printf("\t%s: '%s'", varName, value)
		}
	}
	return nil
}

func List() []string {
	files, err := os.ReadDir(vip.GetString("directory"))
	if err != nil {
		log.Fatal(err)
	}
	var p []string
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".yml" {
			p = append(p, strings.TrimSuffix(file.Name(), filepath.Ext(file.Name())))
		}
	}
	return p
}

func Load(p string) (*vip.Viper, error) {
	log.Println("loading personality:", p)
	conf := vip.New()
	conf.SetConfigFile(vip.GetString("directory") + "/" + p + ".yml")

	err := conf.ReadInConfig()
	if err != nil {
		log.Println("Error reading personality config:", err)
		return nil, err
	}
	return conf, nil
}
