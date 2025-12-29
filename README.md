# igcopy

## 概要

入力ディレクトリの画像ファイルを出力ディレクトリにコピーするツールです。サブディレクトリ以下も再帰的にコピーします。

重複コピーを防止するために出力ディレクトリ側にはディレクトリごとにSQLite3データベース（`<output_directory>/**/igcopy.db`）を配置してファイル名を登録します。重複検証の際は出力ディレクトリ側にファイルが存在しなくてもデータベース上に存在していれば重複とみなします。

## 使い方

```sh
go run main.go -input <input_directory> -output <output_directory>
```

## 例

```sh
go run main.go -input ./input -output ./output
```

