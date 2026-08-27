// genicon 生成 gio-browser 的 Windows ICO 资源文件。
//
// 图标视觉定义与 internal/app 中程序化绘制的行星图标完全同源，
// 生成的 icon.ico 经 rsrc 打包为 rsrc_windows_amd64.syso 后由
// Go 工具链自动链接进 exe，使资源管理器展示应用图标。
//
// 用法：go run ./tools/genicon -out icon.ico
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"gio-browser/internal/app"
)

// icoSizes ICO 内包含的尺寸档位（覆盖任务栏 / 资源管理器 / Alt+Tab 各场景）。
var icoSizes = []int{16, 32, 48, 64, 128, 256}

func main() {
	out := flag.String("out", "icon.ico", "输出 ICO 文件路径")
	flag.Parse()

	// ICO 头（6 字节）+ 每档位一个 16 字节目录项 + PNG 数据
	var buf bytes.Buffer
	buf.Write([]byte{0, 0, 1, 0, byte(len(icoSizes)), 0}) // reserved=0, type=1(icon), count

	// 先编码 PNG，计算偏移后再补目录项
	type entry struct {
		size   int
		data   []byte
		offset uint32
	}
	var entries []entry
	var dataOffset uint32 = 6 + 16*uint32(len(icoSizes))
	for _, s := range icoSizes {
		img := app.IconImage(s)
		var pb bytes.Buffer
		if err := png.Encode(&pb, img); err != nil {
			log.Fatalf("编码 %dpx PNG 失败: %v", s, err)
		}
		entries = append(entries, entry{size: s, data: pb.Bytes(), offset: dataOffset})
		dataOffset += uint32(len(pb.Bytes()))
	}

	for _, e := range entries {
		w, h := byte(e.size), byte(e.size)
		if e.size >= 256 {
			w, h = 0, 0 // ICO 规范：256 记作 0
		}
		buf.Write([]byte{w, h, 0, 0, 1, 0, 32, 0}) // w,h,色板,保留,色面,位深
		writeLE32(&buf, uint32(len(e.data)))
		writeLE32(&buf, e.offset)
	}
	for _, e := range entries {
		buf.Write(e.data)
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatalf("创建输出目录失败: %v", err)
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		log.Fatalf("写入 %s 失败: %v", *out, err)
	}
	fmt.Printf("已生成 %s（%d 个尺寸档位，%d 字节）\n", *out, len(icoSizes), buf.Len())
}

func writeLE32(buf *bytes.Buffer, v uint32) {
	buf.WriteByte(byte(v))
	buf.WriteByte(byte(v >> 8))
	buf.WriteByte(byte(v >> 16))
	buf.WriteByte(byte(v >> 24))
}
