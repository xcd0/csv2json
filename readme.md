# csv2json

## usage

```sh
$ ./csv2json.exe foo.csv
```

```sh
$ cat csv.json | ./csv2json.exe
```

## install

```sh
go install github.com/xcd0/csv2json@latest
```

## 基本

```csv
a,b,c
01,02,03
11,12,13
```

のようなcsvがあるとき、
1行目をキーとして、2行目以降の列をjsonとして出力する。

```sh
$ echo -e "a,b,c\n01,02,03\n11,12,13" | ./csv2json.exe
[
    {
        "a": "01",
        "b": "02",
        "c": "03"
    },
    {
        "a": "11",
        "b": "12",
        "c": "13"
    }
]
```
のようにjsonを出力する。


但し、1行目の列が空の時、自動で列名を生成する。
この自動生成する列名は、`-p`で変更できる。
この自動生成する列名は、`文字列`+`番号`になっており、`文字列`の部分は`-p`で変更できる。

```sh
$ echo -e ",\n01,02,03\n11,12,13" | ./csv2json.exe
[
    {
        "col_0": "01",
        "col_1": "02",
        "col_2": "03"
    },
    {
        "col_0": "11",
        "col_1": "12",
        "col_2": "13"
    }
]
```

もし、1列目の中で重複する場合、自動で文字列を付与する。
この自動生成する列名は、`重複した名前`+`_`+`番号`になっており、`_`の部分は`-s`で変更できる。


```sh
$ echo -e "a,a,a\n01,02,03\n11,12,13" | ./csv2json.exe
[
    {
        "a": "01",
        "a_2": "02",
        "a_3": "03"
    },
    {
        "a": "11",
        "a_2": "12",
        "a_3": "13"
    }
]
```

注意点として、1行目以降連続して空行の場合、読み飛ばされ、空行でない行がjsonのキーになる。
```sh
$ echo -e "\n\n\n\n01,02,03\n11,12,13" | ./csv2json.exe
[
    {
        "01": "11",
        "02": "12",
        "03": "13"
    }
]
```

## おまけ機能

### 行番号出力

`-n 文字列` のように指定すると、jsonにCSV上での元の行番号が出力される。

```sh
$ echo -e "a,b,c\n01,02,03\n11,12,13" | ./csv2json.exe -n line_number
[
    {
        "a": "01",
        "b": "02",
        "c": "03",
        "line_number": 1
    },
    {
        "a": "11",
        "b": "12",
        "c": "13",
        "line_number": 2
    }
]
```

### minify

`-m`でminifyされる。

```sh
$ echo -e "a,b,c\n01,02,03\n11,12,13" | ./csv2json.exe -m
[{"a":"01","b":"02","c":"03"},{"a":"11","b":"12","c":"13"}]
```

### インデント文字指定。

`-i` でインデントに使用する文字列を指定できる。

```sh
$ echo -e "a,b,c\n01,02,03\n11,12,13" | ./csv2json.exe -i ____
[
____{
________"a": "01",
________"b": "02",
________"c": "03"
____},
____{
________"a": "11",
________"b": "12",
________"c": "13"
____}
]
```
