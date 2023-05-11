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
	for _, name := range v.AllKeys() {
		if name == "admins" || name == "discordtoken" {
			continue
		}
		value := v.GetString(name)
		if value == "" {
			return fmt.Errorf("! %s unset. use --%s flag, personality config, or SOULSHACK_%s env", name, name, strings.ToUpper(name))
		}

		if v.GetBool("verbose") {
			if name == "openaikey" || name == "discordtoken" {
				value = strings.Repeat("*", len(value))
			}
			log.Printf("\t%s: '%s'", name, value)
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
	log.Println("loading config:", p)
	conf := vip.New()
	conf.SetConfigType("yml")
	conf.SetConfigFile(filepath.Join(vip.GetString("directory"), p+".yml"))
	err := conf.ReadInConfig()
	if err != nil {
		log.Println("Error reading config:", err)
		return nil, err
	}
	return conf, nil
}
