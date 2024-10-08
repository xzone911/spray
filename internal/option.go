package internal

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/chainreactors/files"
	"github.com/chainreactors/logs"
	"github.com/chainreactors/parsers"
	"github.com/chainreactors/spray/internal/ihttp"
	"github.com/chainreactors/spray/internal/pool"
	"github.com/chainreactors/spray/pkg"
	"github.com/chainreactors/utils"
	"github.com/chainreactors/utils/iutils"
	"github.com/chainreactors/words/mask"
	"github.com/chainreactors/words/rule"
	"github.com/expr-lang/expr"
	"github.com/vbauerster/mpb/v8"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	DefaultThreads = 20
	SkipChar       = "%SKIP%"
)

type Option struct {
	InputOptions    `group:"Input Options" config:"input" `
	FunctionOptions `group:"Function Options" config:"functions" `
	OutputOptions   `group:"Output Options" config:"output"`
	PluginOptions   `group:"Plugin Options" config:"plugins"`
	FingerOptions   `group:"Finger Options" config:"finger"`
	RequestOptions  `group:"Request Options" config:"request"`
	ModeOptions     `group:"Modify Options" config:"mode"`
	MiscOptions     `group:"Miscellaneous Options" config:"misc"`
}

type InputOptions struct {
	ResumeFrom   string   `long:"resume" description:"File, resume filename" `
	Config       string   `short:"c" long:"config" description:"File, config filename"`
	URL          []string `short:"u" long:"url" description:"Strings, input baseurl, e.g.: http://google.com"`
	URLFile      string   `short:"l" long:"list" description:"File, input filename"`
	PortRange    string   `short:"p" long:"port" description:"String, input port range, e.g.: 80,8080-8090,db"`
	CIDRs        []string `short:"i" long:"cidr" description:"String, input cidr, e.g.: 1.1.1.1/24 "`
	RawFile      string   `long:"raw" description:"File, input raw request filename"`
	Dictionaries []string `short:"d" long:"dict" description:"Files, Multi,dict files, e.g.: -d 1.txt -d 2.txt" config:"dictionaries"`
	DefaultDict  bool     `short:"D" long:"default" description:"Bool, use default dictionary" config:"default"`
	Word         string   `short:"w" long:"word" description:"String, word generate dsl, e.g.: -w test{?ld#4}" config:"word"`
	Rules        []string `short:"r" long:"rules" description:"Files, rule files, e.g.: -r rule1.txt -r rule2.txt" config:"rules"`
	AppendRule   []string `long:"append-rule" description:"Files, when found valid path , use append rule generator new word with current path" config:"append-rules"`
	FilterRule   string   `long:"filter-rule" description:"String, filter rule, e.g.: --rule-filter '>8 <4'" config:"filter-rule"`
	AppendFile   []string `long:"append" description:"Files, when found valid path , use append file new word with current path" config:"append-files"`
	Offset       int      `long:"offset" description:"Int, wordlist offset"`
	Limit        int      `long:"limit" description:"Int, wordlist limit, start with offset. e.g.: --offset 1000 --limit 100"`
}

type FunctionOptions struct {
	Extensions        string            `short:"e" long:"extension" description:"String, add extensions (separated by commas), e.g.: -e jsp,jspx" config:"extension"`
	ForceExtension    bool              `long:"force-extension" description:"Bool, force add extensions" config:"force-extension"`
	ExcludeExtensions string            `long:"exclude-extension" description:"String, exclude extensions (separated by commas), e.g.: --exclude-extension jsp,jspx" config:"exclude-extension"`
	RemoveExtensions  string            `long:"remove-extension" description:"String, remove extensions (separated by commas), e.g.: --remove-extension jsp,jspx" config:"remove-extension"`
	Uppercase         bool              `short:"U" long:"uppercase" description:"Bool, upper wordlist, e.g.: --uppercase" config:"upper"`
	Lowercase         bool              `short:"L" long:"lowercase" description:"Bool, lower wordlist, e.g.: --lowercase" config:"lower"`
	Prefixes          []string          `long:"prefix" description:"Strings, add prefix, e.g.: --prefix aaa --prefix bbb" config:"prefix"`
	Suffixes          []string          `long:"suffix" description:"Strings, add suffix, e.g.: --suffix aaa --suffix bbb" config:"suffix"`
	Replaces          map[string]string `long:"replace" description:"Strings, replace string, e.g.: --replace aaa:bbb --replace ccc:ddd" config:"replace"`
	Skips             []string          `long:"skip" description:"String, skip word when generate. rule, e.g.: --skip aaa" config:"skip"`
	//SkipEval          string            `long:"skip-eval" description:"String, skip word when generate. rule, e.g.: --skip-eval 'current.Length < 4'"`
}

type OutputOptions struct {
	Match       string `long:"match" description:"String, custom match function, e.g.: --match 'current.Status != 200''" config:"match" `
	Filter      string `long:"filter" description:"String, custom filter function, e.g.: --filter 'current.Body contains \"hello\"'" config:"filter"`
	Fuzzy       bool   `long:"fuzzy" description:"String, open fuzzy output" config:"fuzzy"`
	OutputFile  string `short:"f" long:"file" description:"String, output filename" json:"output_file,omitempty" config:"output-file"`
	DumpFile    string `long:"dump-file" description:"String, dump all request, and write to filename" config:"dump-file"`
	Dump        bool   `long:"dump" description:"Bool, dump all request" config:"dump"`
	AutoFile    bool   `long:"auto-file" description:"Bool, auto generator output and fuzzy filename" config:"auto-file"`
	Format      string `short:"F" long:"format" description:"String, output format, e.g.: --format 1.json" config:"format"`
	Json        bool   `short:"j" long:"json" description:"Bool, output json" config:"json"`
	OutputProbe string `short:"o" long:"probe" description:"String, output format" config:"output_probe"`
	Quiet       bool   `short:"q" long:"quiet" description:"Bool, Quiet" config:"quiet"`
	NoColor     bool   `long:"no-color" description:"Bool, no color" config:"no-color"`
	NoBar       bool   `long:"no-bar" description:"Bool, No progress bar" config:"no-bar"`
	NoStat      bool   `long:"no-stat" description:"Bool, No stat file output" config:"no-stat"`
}

type RequestOptions struct {
	Method          string   `short:"x" long:"method" default:"GET" description:"String, request method, e.g.: --method POST" config:"method"`
	Headers         []string `long:"header" description:"Strings, custom headers, e.g.: --headers 'Auth: example_auth'" config:"headers"`
	UserAgent       string   `long:"user-agent" description:"String, custom user-agent, e.g.: --user-agent Custom" config:"useragent"`
	RandomUserAgent bool     `long:"random-agent" description:"Bool, use random with default user-agent" config:"random-useragent"`
	Cookie          []string `long:"cookie" description:"Strings, custom cookie" config:"cookies"`
	ReadAll         bool     `long:"read-all" description:"Bool, read all response body" config:"read-all"`
	MaxBodyLength   int64    `long:"max-length" default:"100" description:"Int, max response body length (kb), -1 read-all, 0 not read body, default 100k, e.g. --max-length 1000" config:"max-length"`
}

type PluginOptions struct {
	Advance       bool     `short:"a" long:"advance" description:"Bool, enable all plugin" config:"all" `
	Extracts      []string `long:"extract" description:"Strings, extract response, e.g.: --extract js --extract ip --extract version:(.*?)" config:"extract"`
	ExtractConfig string   `long:"extract-config" description:"String, extract config filename" config:"extract-config"`
	ActivePlugin  bool     `long:"active" description:"Bool, enable active finger path"`
	ReconPlugin   bool     `long:"recon" description:"Bool, enable recon" config:"recon"`
	BakPlugin     bool     `long:"bak" description:"Bool, enable bak found" config:"bak"`
	CommonPlugin  bool     `long:"common" description:"Bool, enable common file found" config:"common"`
	CrawlPlugin   bool     `long:"crawl" description:"Bool, enable crawl" config:"crawl"`
	CrawlDepth    int      `long:"crawl-depth" default:"3" description:"Int, crawl depth" config:"crawl-depth"`
}

type ModeOptions struct {
	RateLimit       int      `long:"rate-limit" default:"0" description:"Int, request rate limit (rate/s), e.g.: --rate-limit 100" config:"rate-limit"`
	Force           bool     `long:"force" description:"Bool, skip error break" config:"force"`
	NoScope         bool     `long:"no-scope" description:"Bool, no scope" config:"no-scope"`
	Scope           []string `long:"scope" description:"String, custom scope, e.g.: --scope *.example.com" config:"scope"`
	Recursive       string   `long:"recursive" default:"current.IsDir()" description:"String,custom recursive rule, e.g.: --recursive current.IsDir()" config:"recursive"`
	Depth           int      `long:"depth" default:"0" description:"Int, recursive depth" config:"depth"`
	Index           string   `long:"index" default:"/" description:"String, custom index path" config:"index"`
	Random          string   `long:"random" default:"" description:"String, custom random path" config:"random"`
	CheckPeriod     int      `long:"check-period" default:"200" description:"Int, check period when request" config:"check-period"`
	ErrPeriod       int      `long:"error-period" default:"10" description:"Int, check period when error" config:"error-period"`
	BreakThreshold  int      `long:"error-threshold" default:"20" description:"Int, break when the error exceeds the threshold" config:"error-threshold"`
	BlackStatus     string   `long:"black-status" default:"400,410" description:"Strings (comma split),custom black status" config:"black-status"`
	WhiteStatus     string   `long:"white-status" default:"200" description:"Strings (comma split), custom white status" config:"white-status"`
	FuzzyStatus     string   `long:"fuzzy-status" default:"500,501,502,503,301,302" description:"Strings (comma split), custom fuzzy status" config:"fuzzy-status"`
	UniqueStatus    string   `long:"unique-status" default:"403,200,404" description:"Strings (comma split), custom unique status" config:"unique-status"`
	Unique          bool     `long:"unique" description:"Bool, unique response" config:"unique"`
	RetryCount      int      `long:"retry" default:"0" description:"Int, retry count" config:"retry"`
	SimhashDistance int      `long:"sim-distance" default:"5" config:"sim-distance"`
}

type MiscOptions struct {
	Mod         string `short:"m" long:"mod" default:"path" choice:"path" choice:"host" description:"String, path/host spray" config:"mod"`
	Client      string `short:"C" long:"client" default:"auto" choice:"fast" choice:"standard" choice:"auto" description:"String, Client type" config:"client"`
	Deadline    int    `long:"deadline" default:"999999" description:"Int, deadline (seconds)" config:"deadline"` // todo 总的超时时间,适配云函数的deadline
	Timeout     int    `short:"T" long:"timeout" default:"5" description:"Int, timeout with request (seconds)" config:"timeout"`
	PoolSize    int    `short:"P" long:"pool" default:"5" description:"Int, Pool size" config:"pool"`
	Threads     int    `short:"t" long:"thread" default:"20" description:"Int, number of threads per pool" config:"thread"`
	Debug       bool   `long:"debug" description:"Bool, output debug info" config:"debug"`
	Version     bool   `long:"version" description:"Bool, show version"`
	Verbose     []bool `short:"v" description:"Bool, log verbose level ,default 0, level1: -v level2 -vv " config:"verbose"`
	Proxy       string `long:"proxy" description:"String, proxy address, e.g.: --proxy socks5://127.0.0.1:1080" config:"proxy"`
	InitConfig  bool   `long:"init" description:"Bool, init config file"`
	PrintPreset bool   `long:"print" description:"Bool, print preset all preset config "`
}

func (opt *Option) Validate() error {
	if opt.Uppercase && opt.Lowercase {
		return errors.New("cannot set -U and -L at the same time")
	}

	if (opt.Offset != 0 || opt.Limit != 0) && opt.Depth > 0 {
		// 偏移和上限与递归同时使用时也会造成混淆.
		return errors.New("--offset and --limit cannot be used with --depth at the same time")
	}

	if opt.Depth > 0 && opt.ResumeFrom != "" {
		// 递归与断点续传会造成混淆, 断点续传的word与rule不是通过命令行获取的

		return errors.New("--resume and --depth cannot be used at the same time")
	}

	if opt.ResumeFrom == "" && len(opt.URL) == 0 && opt.URLFile == "" && len(opt.CIDRs) == 0 && opt.RawFile == "" {
		return fmt.Errorf("without any target, please use -u/-l/-c/--resume to set targets")
	}

	return nil
}

func (opt *Option) Prepare() error {
	var err error
	logs.Log.SetColor(true)
	if err = opt.FingerOptions.Validate(); err != nil {
		return err
	}

	if opt.FingerUpdate {
		err = opt.UpdateFinger()
		if err != nil {
			return err
		}
	}
	err = opt.LoadLocalFingerConfig()
	if err != nil {
		return err
	}

	err = opt.Validate()
	if err != nil {
		return err
	}
	err = pkg.LoadFingers()
	if err != nil {
		return err
	}

	if opt.Extracts != nil {
		for _, e := range opt.Extracts {
			if reg, ok := pkg.ExtractRegexps[e]; ok {
				pkg.Extractors[e] = reg
			} else {
				pkg.Extractors[e] = []*parsers.Extractor{
					&parsers.Extractor{
						Name:            e,
						CompiledRegexps: []*regexp.Regexp{regexp.MustCompile(e)},
					},
				}
			}
		}
	}
	if opt.ExtractConfig != "" {
		extracts, err := pkg.LoadExtractorConfig(opt.ExtractConfig)
		if err != nil {
			return err
		}
		pkg.Extractors[opt.ExtractConfig] = extracts
	}

	err = pkg.Load()
	if err != nil {
		iutils.Fatal(err.Error())
	}

	// 初始化全局变量
	pkg.Distance = uint8(opt.SimhashDistance)
	if opt.MaxBodyLength == -1 {
		ihttp.DefaultMaxBodySize = -1
	} else {
		ihttp.DefaultMaxBodySize = opt.MaxBodyLength * 1024
	}

	pkg.BlackStatus = pkg.ParseStatus(pkg.BlackStatus, opt.BlackStatus)
	pkg.WhiteStatus = pkg.ParseStatus(pkg.WhiteStatus, opt.WhiteStatus)
	if opt.FuzzyStatus == "all" {
		pool.EnableAllFuzzy = true
	} else {
		pkg.FuzzyStatus = pkg.ParseStatus(pkg.FuzzyStatus, opt.FuzzyStatus)
	}

	if opt.Unique {
		pool.EnableAllUnique = true
	} else {
		pkg.UniqueStatus = pkg.ParseStatus(pkg.UniqueStatus, opt.UniqueStatus)
	}
	pool.MaxCrawl = opt.CrawlDepth

	return nil
}

func (opt *Option) NewRunner() (*Runner, error) {
	var err error
	r := &Runner{
		Option:   opt,
		taskCh:   make(chan *Task),
		outputCh: make(chan *pkg.Baseline, 256),
		poolwg:   &sync.WaitGroup{},
		outwg:    &sync.WaitGroup{},
		fuzzyCh:  make(chan *pkg.Baseline, 256),
		Headers:  make(map[string]string),
		Total:    opt.Limit,
		Color:    true,
	}

	// log and bar
	if opt.NoColor {
		logs.Log.SetColor(false)
		r.Color = false
	}
	if opt.Quiet {
		logs.Log.SetQuiet(true)
		logs.Log.SetColor(false)
		r.Color = false
	}

	if !(opt.Quiet || opt.NoBar) {
		r.Progress = mpb.New(mpb.WithRefreshRate(100 * time.Millisecond))
		logs.Log.SetOutput(r.Progress)
	}

	// configuration
	if opt.Force {
		// 如果开启了force模式, 将关闭check机制, err积累到一定数量自动退出机制
		r.BreakThreshold = MAX
		r.CheckPeriod = MAX
		r.ErrPeriod = MAX
	}

	// 选择client
	if opt.Client == "auto" {
		r.ClientType = ihttp.Auto
	} else if opt.Client == "fast" {
		r.ClientType = ihttp.FAST
	} else if opt.Client == "standard" || opt.Client == "base" || opt.Client == "http" {
		r.ClientType = ihttp.STANDARD
	}

	if opt.Threads == DefaultThreads && len(opt.Dictionaries) == 0 {
		r.Threads = 1000
	}

	err = opt.BuildPlugin(r)
	if err != nil {
		return nil, err
	}

	err = opt.BuildWords(r)
	if err != nil {
		return nil, err
	}

	pkg.DefaultStatistor = pkg.Statistor{
		Word:         opt.Word,
		WordCount:    len(r.Wordlist),
		Dictionaries: opt.Dictionaries,
		Offset:       opt.Offset,
		RuleFiles:    opt.Rules,
		RuleFilter:   opt.FilterRule,
		Total:        r.Total,
	}

	r.Tasks, err = opt.BuildTasks(r)
	if err != nil {
		return nil, err
	}

	if opt.Match != "" {
		exp, err := expr.Compile(opt.Match)
		if err != nil {
			return nil, err
		}
		r.MatchExpr = exp
	}

	if opt.Filter != "" {
		exp, err := expr.Compile(opt.Filter)
		if err != nil {
			return nil, err
		}
		r.FilterExpr = exp
	}

	// 初始化递归
	var express string
	if opt.Recursive != "current.IsDir()" && opt.Depth != 0 {
		// 默认不打开递归, 除非指定了非默认的递归表达式
		pool.MaxRecursion = 1
		express = opt.Recursive
	}

	if opt.Depth != 0 {
		// 手动设置的depth优先级高于默认
		pool.MaxRecursion = opt.Depth
		express = opt.Recursive
	}

	if express != "" {
		exp, err := expr.Compile(express)
		if err != nil {
			return nil, err
		}
		r.RecursiveExpr = exp
	}

	// prepare header
	for _, h := range opt.Headers {
		i := strings.Index(h, ":")
		if i == -1 {
			logs.Log.Warn("invalid header")
		} else {
			r.Headers[h[:i]] = h[i+2:]
		}
	}

	if opt.UserAgent != "" {
		r.Headers["User-Agent"] = opt.UserAgent
	}
	if opt.Cookie != nil {
		r.Headers["Cookie"] = strings.Join(opt.Cookie, "; ")
	}

	if opt.OutputProbe != "" {
		r.Probes = strings.Split(opt.OutputProbe, ",")
	}

	// init output file
	if opt.OutputFile != "" {
		r.OutputFile, err = files.NewFile(opt.OutputFile, false, false, true)
		if err != nil {
			return nil, err
		}
	} else if opt.AutoFile {
		r.OutputFile, err = files.NewFile("result.json", false, false, true)
		if err != nil {
			return nil, err
		}
	}

	//if opt.FuzzyFile != "" {
	//	r.FuzzyFile, err = files.NewFile(opt.FuzzyFile, false, false, true)
	//	if err != nil {
	//		return nil, err
	//	}
	//} else if opt.AutoFile {
	//	r.FuzzyFile, err = files.NewFile("fuzzy.json", false, false, true)
	//	if err != nil {
	//		return nil, err
	//	}
	//}

	if opt.DumpFile != "" {
		r.DumpFile, err = files.NewFile(opt.DumpFile, false, false, true)
		if err != nil {
			return nil, err
		}
	} else if opt.Dump {
		r.DumpFile, err = files.NewFile("dump.json", false, false, true)
		if err != nil {
			return nil, err
		}
	}
	if opt.ResumeFrom != "" {
		r.StatFile, err = files.NewFile(opt.ResumeFrom, false, true, true)
	} else {
		r.StatFile, err = files.NewFile(pkg.SafeFilename(r.Tasks.Name)+".stat", false, true, true)
	}
	if err != nil {
		return nil, err
	}

	if !opt.NoStat {
		r.StatFile.Mod = os.O_WRONLY | os.O_CREATE
		err = r.StatFile.Init()
		if err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (opt *Option) PrintPlugin() {
	var s strings.Builder
	if opt.CrawlPlugin {
		s.WriteString("crawl enable; ")
	}
	if opt.Finger {
		s.WriteString("active fingerprint enable; ")
	}
	if opt.BakPlugin {
		s.WriteString("bak file enable; ")
	}
	if opt.CommonPlugin {
		s.WriteString("common file enable; ")
	}
	if opt.ReconPlugin {
		s.WriteString("recon enable; ")
	}
	if opt.RetryCount > 0 {
		s.WriteString("Retry Count: " + strconv.Itoa(opt.RetryCount))
	}

	if s.Len() > 0 {
		logs.Log.Important(s.String())
	}
}

func (opt *Option) BuildPlugin(r *Runner) error {
	// brute only
	if opt.Advance {
		opt.CrawlPlugin = true
		opt.Finger = true
		opt.BakPlugin = true
		opt.CommonPlugin = true
		opt.ActivePlugin = true
		opt.ReconPlugin = true
	}

	if opt.ReconPlugin {
		pkg.Extractors["recon"] = pkg.ExtractRegexps["pentest"]
	}

	if opt.Finger {
		pkg.EnableAllFingerEngine = true
	}

	if opt.BakPlugin {
		r.bruteMod = true
		opt.AppendRule = append(opt.AppendRule, "filebak")
		r.AppendWords = append(r.AppendWords, pkg.GetPresetWordList([]string{"bak_file"})...)
	}

	if opt.CommonPlugin {
		r.bruteMod = true
		r.AppendWords = append(r.AppendWords, pkg.Dicts["common"]...)
		r.AppendWords = append(r.AppendWords, pkg.Dicts["log"]...)
	}

	if opt.ActivePlugin {
		r.bruteMod = true
		r.AppendWords = append(r.AppendWords, pkg.ActivePath...)
	}

	if opt.CrawlPlugin {
		r.bruteMod = true
	}

	opt.PrintPlugin()
	if r.bruteMod {
		logs.Log.Important("enabling brute mod, because of enabled brute plugin")
	}

	if opt.NoScope {
		r.Scope = []string{"*"}
	}
	return nil
}

func (opt *Option) BuildWords(r *Runner) error {
	var dicts [][]string
	var err error
	if opt.DefaultDict {
		dicts = append(dicts, pkg.Dicts["default"])
		logs.Log.Info("use default dictionary: https://github.com/maurosoria/dirsearch/blob/master/db/dicc.txt")
	}
	for i, f := range opt.Dictionaries {
		dict, err := pkg.LoadFileToSlice(f)
		if err != nil {
			return err
		}
		dicts = append(dicts, dict)
		if opt.ResumeFrom != "" {
			pkg.Dicts[f] = dicts[i]
		}

		logs.Log.Logf(pkg.LogVerbose, "Loaded %d word from %s", len(dict), f)
	}

	if len(dicts) == 0 && opt.Word == "" && len(opt.Rules) == 0 && len(opt.AppendRule) == 0 {
		r.IsCheck = true
	}

	if opt.Word == "" {
		opt.Word = "{?"
		for i, _ := range dicts {
			opt.Word += strconv.Itoa(i)
		}
		opt.Word += "}"
	}

	if len(opt.Suffixes) != 0 {
		mask.SpecialWords["suffix"] = opt.Suffixes
		opt.Word += "{?@suffix}"
	}
	if len(opt.Prefixes) != 0 {
		mask.SpecialWords["prefix"] = opt.Prefixes
		opt.Word = "{?@prefix}" + opt.Word
	}

	if opt.ForceExtension && opt.Extensions != "" {
		exts := strings.Split(opt.Extensions, ",")
		for i, e := range exts {
			if !strings.HasPrefix(e, ".") {
				exts[i] = "." + e
			}
		}
		mask.SpecialWords["ext"] = exts
		opt.Word += "{?@ext}"
	}

	r.Wordlist, err = mask.Run(opt.Word, dicts, nil)
	if err != nil {
		return fmt.Errorf("%s %w", opt.Word, err)
	}
	if len(r.Wordlist) > 0 {
		logs.Log.Logf(pkg.LogVerbose, "Parsed %d words by %s", len(r.Wordlist), opt.Word)
	}

	if len(opt.Rules) != 0 {
		rules, err := pkg.LoadRuleAndCombine(opt.Rules)
		if err != nil {
			return err
		}
		r.Rules = rule.Compile(rules, opt.FilterRule)
	} else if opt.FilterRule != "" {
		// if filter rule is not empty, set rules to ":", force to open filter mode
		r.Rules = rule.Compile(":", opt.FilterRule)
	} else {
		r.Rules = new(rule.Program)
	}

	if len(r.Rules.Expressions) > 0 {
		r.Total = len(r.Wordlist) * len(r.Rules.Expressions)
	} else {
		r.Total = len(r.Wordlist)
	}

	if len(opt.AppendRule) != 0 {
		content, err := pkg.LoadRuleAndCombine(opt.AppendRule)
		if err != nil {
			return err
		}
		r.AppendRules = rule.Compile(string(content), "")
	}

	if len(opt.AppendFile) != 0 {
		var lines []string
		for _, f := range opt.AppendFile {
			dict, err := pkg.LoadFileToSlice(f)
			if err != nil {
				return err
			}
			lines = append(lines, dict...)
		}
		r.AppendWords = append(r.AppendWords, lines...)
	}

	//  类似dirsearch中的
	if opt.Extensions != "" {
		r.AppendFunction(func(s string) []string {
			exts := strings.Split(opt.Extensions, ",")
			ss := make([]string, len(exts))
			for i, e := range exts {
				if strings.Contains(s, "%EXT%") {
					ss[i] = strings.Replace(s, "%EXT%", e, -1)
				}
			}
			return ss
		})
	} else {
		r.AppendFunction(func(s string) []string {
			if strings.Contains(s, "%EXT%") {
				return nil
			}
			return []string{s}
		})
	}

	if opt.Uppercase {
		r.AppendFunction(pkg.WrapWordsFunc(strings.ToUpper))
	}
	if opt.Lowercase {
		r.AppendFunction(pkg.WrapWordsFunc(strings.ToLower))
	}

	if opt.RemoveExtensions != "" {
		rexts := strings.Split(opt.ExcludeExtensions, ",")
		r.AppendFunction(func(s string) []string {
			if ext := pkg.ParseExtension(s); iutils.StringsContains(rexts, ext) {
				return []string{strings.TrimSuffix(s, "."+ext)}
			}
			return []string{s}
		})
	}

	if opt.ExcludeExtensions != "" {
		exexts := strings.Split(opt.ExcludeExtensions, ",")
		r.AppendFunction(func(s string) []string {
			if ext := pkg.ParseExtension(s); iutils.StringsContains(exexts, ext) {
				return nil
			}
			return []string{s}
		})
	}

	if len(opt.Replaces) > 0 {
		r.AppendFunction(func(s string) []string {
			for k, v := range opt.Replaces {
				s = strings.Replace(s, k, v, -1)
			}
			return []string{s}
		})
	}

	// default skip function, skip %EXT%
	r.AppendFunction(func(s string) []string {
		if strings.Contains(s, "%EXT%") {
			return nil
		}
		return []string{s}
	})
	if len(opt.Skips) > 0 {
		r.AppendFunction(func(s string) []string {
			for _, skip := range opt.Skips {
				if strings.Contains(s, skip) {
					return nil
				}
			}
			return []string{s}
		})
	}

	logs.Log.Importantf("Loaded %d dictionaries, %d rules and %d decorators", len(opt.Dictionaries), len(opt.Rules), len(r.Fns))
	return nil
}

func (opt *Option) BuildTasks(r *Runner) (*TaskGenerator, error) {
	// prepare task`
	var err error
	gen := NewTaskGenerator(opt.PortRange)
	if opt.ResumeFrom != "" {
		stats, err := pkg.ReadStatistors(opt.ResumeFrom)
		if err != nil {
			logs.Log.Error(err.Error())
		}
		r.Count = len(stats)
		gen.Name = "resume " + opt.ResumeFrom
		go func() {
			for _, stat := range stats {
				gen.In <- &Task{baseUrl: stat.BaseUrl, origin: NewOrigin(stat)}
			}
			close(gen.In)
		}()
	} else {
		var file *os.File

		// 根据不同的输入类型生成任务
		if len(opt.URL) == 1 {
			gen.Name = opt.URL[0]
			go func() {
				gen.Run(opt.URL[0])
				close(gen.In)
			}()
			r.Count = 1
		} else if len(opt.URL) > 1 {
			go func() {
				for _, u := range opt.URL {
					gen.Run(u)
				}
				close(gen.In)
			}()
			gen.Name = "cmd"
			r.Count = len(opt.URL)
		} else if opt.RawFile != "" {
			raw, err := os.Open(opt.RawFile)
			if err != nil {
				return nil, err
			}

			req, err := http.ReadRequest(bufio.NewReader(raw))
			if err != nil {
				return nil, err
			}
			go func() {
				gen.Run(fmt.Sprintf("http://%s%s", req.Host, req.URL.String()))
				close(gen.In)
			}()
			r.Method = req.Method
			for k, _ := range req.Header {
				r.Headers[k] = req.Header.Get(k)
			}
			r.Count = 1
		} else if len(opt.CIDRs) != 0 {
			cidrs := utils.ParseCIDRs(opt.CIDRs)
			if len(gen.ports) == 0 {
				gen.ports = []string{"80", "443"}
			}
			gen.Name = "cidr"
			r.Count = cidrs.Count()
			go func() {
				for _, cidr := range cidrs {
					if cidr == nil {
						logs.Log.Error("cidr format error: " + cidr.String())
					}
					for ip := range cidr.Range() {
						gen.Run(ip.String())
					}
				}
				close(gen.In)
			}()
		} else if opt.URLFile != "" {
			file, err = os.Open(opt.URLFile)
			if err != nil {
				return nil, err
			}
			gen.Name = filepath.Base(opt.URLFile)
		} else if files.HasStdin() {
			file = os.Stdin
			gen.Name = "stdin"
		}
		if file != nil {
			content, err := ioutil.ReadAll(file)
			if err != nil {
				return nil, err
			}
			urls := strings.Split(strings.TrimSpace(string(content)), "\n")
			for _, u := range urls {
				u = strings.TrimSpace(u)
				if _, err := url.Parse(u); err == nil {
					r.Count++
				} else if ip := utils.ParseIP(u); ip != nil {
					r.Count++
				} else if cidr := utils.ParseCIDR(u); cidr != nil {
					r.Count += cidr.Count()
				}
			}

			go func() {
				for _, u := range urls {
					u = strings.TrimSpace(u)
					if _, err := url.Parse(u); err == nil {
						gen.Run(u)
					} else if ip := utils.ParseIP(u); ip != nil {
						gen.Run(u)
					} else if cidr := utils.ParseCIDR(u); cidr != nil {
						for ip := range cidr.Range() {
							gen.Run(ip.String())
						}
					}
				}
				close(gen.In)
			}()
		}
	}

	if len(gen.ports) > 0 {
		r.Count = r.Count * len(gen.ports)
	}
	return gen, nil
}
