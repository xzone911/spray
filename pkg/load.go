package pkg

import (
	"encoding/json"
	"github.com/chainreactors/fingers"
	"github.com/chainreactors/parsers"
	"github.com/chainreactors/utils"
	"github.com/chainreactors/utils/iutils"
	"github.com/chainreactors/words/mask"
	"os"
	yaml "sigs.k8s.io/yaml/goyaml.v3"
	"strings"
)

var (
	ExtractRegexps = make(parsers.Extractors)
	Extractors     = make(parsers.Extractors)

	FingerEngine *fingers.Engine
	ActivePath   []string
)

func LoadPorts() error {
	var err error
	var ports []*utils.PortConfig
	err = json.Unmarshal(LoadConfig("port"), &ports)
	if err != nil {
		return err
	}
	utils.PrePort = utils.NewPortPreset(ports)
	return nil
}

func LoadFingers() error {
	var err error
	FingerEngine, err = fingers.NewEngine()
	if err != nil {
		return err
	}
	for _, f := range FingerEngine.Fingers().HTTPFingers {
		for _, rule := range f.Rules {
			if rule.SendDataStr != "" {
				ActivePath = append(ActivePath, rule.SendDataStr)
			}
		}
	}
	for _, f := range FingerEngine.FingerPrintHub().FingerPrints {
		if f.Path != "/" {
			ActivePath = append(ActivePath, f.Path)
		}
	}
	return nil
}

func LoadTemplates() error {
	var err error
	// load rule
	var data map[string]interface{}
	err = json.Unmarshal(LoadConfig("spray_rule"), &data)
	if err != nil {
		return err
	}
	for k, v := range data {
		Rules[k] = v.(string)
	}

	// load mask
	var keywords map[string]interface{}
	err = json.Unmarshal(LoadConfig("spray_common"), &keywords)
	if err != nil {
		return err
	}

	for k, v := range keywords {
		t := make([]string, len(v.([]interface{})))
		for i, vv := range v.([]interface{}) {
			t[i] = iutils.ToString(vv)
		}
		mask.SpecialWords[k] = t
	}

	var extracts []*parsers.Extractor
	err = json.Unmarshal(LoadConfig("extract"), &extracts)
	if err != nil {
		return err
	}

	for _, extract := range extracts {
		extract.Compile()

		ExtractRegexps[extract.Name] = []*parsers.Extractor{extract}
		for _, tag := range extract.Tags {
			if _, ok := ExtractRegexps[tag]; !ok {
				ExtractRegexps[tag] = []*parsers.Extractor{extract}
			} else {
				ExtractRegexps[tag] = append(ExtractRegexps[tag], extract)
			}
		}
	}
	return nil
}

func LoadExtractorConfig(filename string) ([]*parsers.Extractor, error) {
	var extracts []*parsers.Extractor
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(content, &extracts)
	if err != nil {
		return nil, err
	}

	for _, extract := range extracts {
		extract.Compile()
	}

	return extracts, nil
}

func Load() error {
	err := LoadPorts()
	if err != nil {
		return err
	}
	err = LoadTemplates()
	if err != nil {
		return err
	}

	return nil
}

func LoadDefaultDict() []string {
	return strings.Split(strings.TrimSpace(string(LoadConfig("spray_default"))), "\n")
}
