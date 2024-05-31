//go:generate go run generate_embed_list.go
package main

import (
	"bytes"
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/pkg/errors"
)

var (
	Version  string = "0.0.1"
	Revision        = func() string { // {{{
		revision := ""
		modified := false
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					//return setting.Value
					revision = setting.Value
					if len(setting.Value) > 7 {
						revision = setting.Value[:7] // 最初の7文字にする
					}
				}
				if setting.Key == "vcs.modified" {
					modified = setting.Value == "true"
				}
			}
		}
		if modified {
			revision = "develop+" + revision
		}
		return revision
	}() // }}}

	embeddedFiles *embed.FS // go:generateによって生成されるembedded_files.goの中で呼び出されるinit()で代入される。
	debug_mode    = false   // ログ出力を出す。
)

func main() {

	if debug_mode {
		log.SetFlags(log.Ltime | log.Lshortfile)
	} else {
		log.SetOutput(io.Discard)
	}

	args := argparse()

	if len(args.Input) == 0 {
		FromStdin(args) // 標準入力から読み取り、標準出力に出力する。
		return
	} else {
		for _, path := range args.Input { // 引数で与えられたファイルを1つづつ処理する。
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("Recovered from: %w", rec)
				}
			}()

			func() {
				c := readcsv(path)
				j := csvToJson(c, args)

				ext := filepath.Ext(path)
				//name := path[:len(path)-len(ext)] + "_output" + ext
				name := path[:len(path)-len(ext)] + "_output.json"
				if args.Debug {
					log.Printf("%v -> %v", path, name)
				}
				if err := os.RemoveAll(name); err != nil {
					panic(errors.Errorf("%v", err))
				}
				if err := os.WriteFile(name, []byte(j), 0644); err != nil {
					panic(errors.Errorf("%v", err))
				}
			}()
		}
	}
}

func readcsv(path string) [][]string {
	file, err := os.Open(path)
	if err != nil {
		panic(errors.Errorf("%v", err))
	}
	defer file.Close()
	r := csv.NewReader(file)
	r.FieldsPerRecord = -1 // FieldsPerRecord を負の値に設定して、CSV リーダーでのレコード長テストを無効にします。
	v, err := r.ReadAll()  // csvを一度に全て読み込む
	if err != nil {
		panic(errors.Errorf("%v", err))
	}
	return v
}

// 配列の内容を解析し、キーとインデックスを返す関数
func arrayContentMatch(str string) (string, int) {
	i := strings.Index(str, "[") // "["の位置を見つける
	if i >= 0 {
		j := strings.Index(str, "]") // "]"の位置を見つける
		if j >= 0 {
			index, _ := strconv.Atoi(str[i+1 : j]) // インデックス部分を整数に変換
			return str[0:i], index                 // キーとインデックスを返す
		}
	}
	return str, -1 // 配列でない場合はインデックスとして-1を返す
}

// 勝手に生成する列名。:で始まっている列名は自動生成されたものであることにする。
func genEmptyColName(i int, args *Args) string {
	return fmt.Sprintf("%v%v", args.Prefix, i)
}

// CSVデータをJSON形式に変換する関数
func csvToJson(rows [][]string, args *Args) string {
	var entries []map[string]interface{} // エントリを格納するスライス
	attributes := rows[0]                // 最初の行を属性名とする

	{ // 最大列数を調べ、最大列数が1行目の列数を超えている場合、attributesに追加する。
		mcol := 0
		for _, row := range rows {
			mcol = max(mcol, len(row))
		}
		if args.Debug {
			log.Printf("rows: %v x %v", len(rows), mcol)
		}
		for i := len(attributes); i < mcol; i++ { // 行の要素数が属性の数より少ない場合
			attributes = append(attributes, genEmptyColName(i, args)) // 不足している列分追加する。
			if args.Debug {
				log.Printf("%v を追加しました。", attributes[len(attributes)-1])
			}
		}
		for i, v := range attributes {
			if len(v) == 0 {
				attributes[i] = genEmptyColName(i, args) // 1行目で、空の要素があるときは勝手に名前を生成する。
			}
		}
	}
	{ // attributes内に同じ文字列が含まれているときjsonのキーとして使用すると競合し、上書きされてしまうため、不適切である。その為、同値の列名はsuffix+数字を付与する。
		c := map[string]int{}
		for i, v := range attributes {
			key := fmt.Sprintf("%v", v)
			if _, exist := c[key]; !exist {
				c[key] = 0
			}
			c[key]++
			if c[key] != 1 {
				// 競合しているのでsuffixを付与する。
				attributes[i] = fmt.Sprintf("%v%v%v", key, args.Suffix, c[key])
			}
		}
		if _, exist := c[args.LineNumber]; exist {
			// CSV上での行番号を出力するキーが競合した場合も同様にsuffixを付与する。
			args.LineNumber = fmt.Sprintf("%v%v%v", args.LineNumber, args.Suffix, c[args.LineNumber])
		}

	}

	if args.Debug {
		log.Printf("Attributes: %v\n", attributes)
	}
	for rowIndex, row := range rows[1:] {
		if args.Debug {
			log.Printf("====================================================================================================")
			log.Printf("rowIndex: %v, row: %v", rowIndex, row)
		}
		entry := map[string]interface{}{} // 各行のデータを格納するマップ
		if args.Debug {
			log.Printf("Processing row %d: %v\n", rowIndex+1, row)
		}
		for i, value := range row {
			if args.Debug {
				log.Printf("i: %d, value: %s, len(attributes): %v\n", i, value, len(attributes))
			}
			if i >= len(attributes) {
				mcol := 0
				for _, row := range rows {
					mcol = max(mcol, len(row))
				}
				if args.Debug {
					log.Printf("rows: %v x %v", len(rows), mcol)
				}
			}
			attribute := attributes[i]
			if args.Debug {
				log.Printf("Processing attribute %d: %s = %s\n", i, attribute, value)
			}
			// CSVヘッダーキーをネストされたオブジェクト用に分割
			objectSlice := strings.Split(attribute, ".")
			internal := entry
			for index, val := range objectSlice {
				// CSVヘッダーキーを配列オブジェクト用に分割
				key, arrayIndex := arrayContentMatch(val)
				if arrayIndex != -1 {
					if internal[key] == nil {
						internal[key] = []interface{}{}
					}
					internalArray := internal[key].([]interface{})
					if index == len(objectSlice)-1 {
						internalArray = append(internalArray, value)
						internal[key] = internalArray
						break
					}
					if arrayIndex >= len(internalArray) {
						internalArray = append(internalArray, map[string]interface{}{})
					}
					internal[key] = internalArray
					internal = internalArray[arrayIndex].(map[string]interface{})
				} else {
					if index == len(objectSlice)-1 {
						internal[key] = value
						break
					}
					if internal[key] == nil {
						internal[key] = map[string]interface{}{}
					}
					internal = internal[key].(map[string]interface{})
				}
			}
		}
		if len(args.LineNumber) != 0 {
			entry[args.LineNumber] = rowIndex + 1 // 元CSVの何行目だったかを埋め込む。1行目が含まれていないので、+1オフセットする。
		}
		entries = append(entries, entry)
	}
	indent := fmt.Sprintf("%v", args.Indent)
	data, err := json.MarshalIndent(entries, "", indent) // JSON形式に整形
	if err != nil {
		panic(errors.Errorf("Marshal error %s\n", err)) // エラーがあればパニック
	}
	if args.Minify {
		dst := &bytes.Buffer{}
		if err := json.Compact(dst, data); err != nil {
			panic(err)
		}
		return dst.String()
	} else {
		return string(data) // JSON文字列を返す
	}

}

func FromStdin(args *Args) {

	if args.Debug {
		log.Printf("%v", args)
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(errors.Errorf("%v", err))
	}
	reader := bytes.NewReader(data)
	r := csv.NewReader(reader)
	r.FieldsPerRecord = -1 // FieldsPerRecord を負の値に設定して、CSV リーダーでのレコード長テストを無効にします。
	v, err := r.ReadAll()  // csvを一度に全て読み込む
	if err != nil {
		panic(errors.Errorf("%v", err))
	}
	j := csvToJson(v, args)
	fmt.Printf("%v\n", j)
}

func GetText(path string) string {
	b, err := os.ReadFile(path) // https://pkg.go.dev/os@go1.20.5#ReadFile
	if err != nil {
		panic(errors.Errorf("Error: %v, file: %v", err, path))
	}
	str := string(b)
	return str
}

func GetFileNameWithoutExt(path string) string {
	return filepath.Base(path[:len(path)-len(filepath.Ext(path))])
}
func GetFilePathWithoutExt(path string) string {
	return filepath.ToSlash(filepath.Join(filepath.Dir(path), GetFileNameWithoutExt(path)))
}
func replaceExt(filePath, from, to string) string {
	ext := filepath.Ext(filePath)
	if len(from) > 0 && ext != from {
		return filePath
	}
	return filePath[:len(filePath)-len(ext)] + to
}

func (args *Args) Print() {
	//	log.Printf(`
	//
	// Csv  : %v
	// Row  : %v
	// Col  : %v
	// Grep : %v
	// `, args.Csv, args.Row, args.Col, args.Grep)
}

// ShowHelp() で使う
var parser *arg.Parser

func ShowHelp(post string) {
	buf := new(bytes.Buffer)
	parser.WriteHelp(buf)
	fmt.Printf("%v\n", strings.ReplaceAll(buf.String(), "display this help and exit", "ヘルプを出力する。"))
	if len(post) != 0 {
		fmt.Println(post)
	}
	os.Exit(1)
}
func ShowVersion() {
	if len(Revision) == 0 {
		// go installでビルドされた場合、gitの情報がなくなる。その場合v0.0.0.のように末尾に.がついてしまうのを避ける。
		fmt.Printf("%v version %v\n", GetFileNameWithoutExt(os.Args[0]), Version)
	} else {
		fmt.Printf("%v version %v.%v\n", GetFileNameWithoutExt(os.Args[0]), Version, Revision)
	}
	os.Exit(0)
}

func argparse() *Args {
	args := &Args{}
	var err error
	parser, err = arg.NewParser(arg.Config{Program: GetFileNameWithoutExt(os.Args[0]), IgnoreEnv: false}, args)
	if err != nil {
		ShowHelp(fmt.Sprintf("%v", errors.Errorf("%v", err)))
	}
	if err := parser.Parse(os.Args[1:]); err != nil {
		if err.Error() == "help requested by user" {
			ShowHelp("")
		} else if err.Error() == "version requested by user" {
			ShowVersion()
		} else {
			panic(errors.Errorf("%v", err))
		}
	}
	//if args.Version || args.VersionSub != nil {
	if args.Version {
		ShowVersion()
	}
	if args.Debug {
		args.Print()
	}
	if args.Code {
		WriteEmbeddedData("./code")
		os.Exit(0)
	}
	return args
}

func WriteEmbeddedData(outputPath string) {
	err := fs.WalkDir(*embeddedFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return errors.Errorf("failed to access %s: %v", path, err)
		}
		if d.IsDir() {
			return nil
		}
		data, err := embeddedFiles.ReadFile(path)
		if err != nil {
			return errors.Errorf("failed to read embedded file %s: %v", path, err)
		}
		outPath := filepath.ToSlash(filepath.Join(outputPath, path))
		if err := os.MkdirAll(filepath.Dir(outPath), os.ModePerm); err != nil {
			return errors.Errorf("failed to create directories for %s: %v", outPath, err)
		}
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return errors.Errorf("failed to write file %s: %v", outPath, err)
		}
		log.Printf("%v", outPath)
		return nil
	})
	if err != nil {
		panic(errors.Errorf("failed to walk embedded files: %v", err))
	}
}

type Args struct {
	Input      []string `arg:"positional"     help:"入力ファイル。"`
	Prefix     string   `arg:"-p,--prefix"    help:"自動生成した列名に付与するprefix。\n                         前提としてこのプログラムで生成するjsonは行ごとにjsonを生成する。\n                         この時入力csvの1行目をjsonのキーとする。\n                         1行目の要素が空の時、キーとして使用できないため、列名を自動生成する。\n                         自動生成した列名は他の列名と衝突しないようにprefixを付与する。\n                         指定がないとき'col_'を使用する。3列目が空の時、'col_3'のように列名が生成される。" default:"col_"`
	Suffix     string   `arg:"-s,--suffix"    help:"自動生成した列名に付与するsuffix。\n                         列名が競合しているとき、自動でsuffixを付与する。指定がないとき_を使用する。\n                         競合している列名'name'が存在するとき3個目の'name'は'name_3'になる。" default:"_"`
	LineNumber string   `arg:"-n,--number"    help:"jsonにCSV上での行番号を出力するキーの指定。\n                         空文字列\"\"を指定した場合、行番号を出力しない。\n                         ':line_number'のように指定すると ':line_number'というキーで行番号が出力される。\n                         指定がないとき、空文字列の時出力しない。" default:""`
	Minify     bool     `arg:"-m,--minify"    help:"出力するjsonをminifyする。"`
	Indent     string   `arg:"-i,--indent"    help:"インデントに使用する文字列を指定する。指定がないときtabを使用する。" default:"\t"`
	Debug      bool     `arg:"-d,--debug"     help:"デバッグ用フラグ。ログが詳細になる。"`
	Version    bool     `arg:"-v,--version"   help:"バージョン情報を出力する。"`
	Code       bool     `arg:"-c,--code"      help:"このプログラムのソースコードを出力する。./codeディレクトリに出力される。"`
}
