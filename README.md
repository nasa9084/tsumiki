# tsumiki

Go製のコードCAD。Fusionのように操作(スケッチ → 押し出し/回転 → 結合/切り取り)を順に積み重ねる履歴型APIで、
3D印刷用の3MF/STLを出力する。カーネルも自作(STEPは非対応、内部表現はメッシュ)。

```go
p := tsumiki.NewPart("plate")
p.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Circle(0, 0, 65), 1.5)
p.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Circle(0, 0, 22.5), 1.5, tsumiki.Cut())
p.Mesh().Save3MF("plate.3mf", "plate")
fmt.Print(p.Timeline())
```

## 機能

- 2Dスケッチ: 多角形・円・角丸四角・線/円弧パス(入れ子はeven-oddで穴になる)
- 押し出し(テーパー付き)・回転、結合/切り取り/交差
- 3MF / バイナリSTL出力、水密性・体積の検証

## ブール演算

平面ベースの厳密BSP。入力を1µm格子に丸め、整数の平面と辺平面で多角形を表現して符号判定を厳密に行うため、
共面・接触ケースでも長い操作の連鎖でも隙間(クラック)が出ない。ランダム生成した共面だらけの形状で
集合代数の体積恒等式を検証している(`stress_test.go`)。

制約: 座標は±500 mm、1 µm丸め。大きなメッシュへのブール1回は数百ms。

## 例

`cmd/guard`: プレステラ90用ガード(364個の六角穴・100個の凹みを含む半割り板付き部品)を13工程で再現する。

```sh
go run ./cmd/guard ./out   # out/ に 3MF と STL を書き出す
```

## テスト

```sh
go test ./...
STRESS_N=400 go test -run TestBooleanRandom -v
```
