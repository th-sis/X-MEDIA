// Package drivers 通过空导入聚合所有驱动，触发各驱动的 init() 注册。
// §13.1 裁剪后仅保留 5 网盘 + LocalFs（移除 OneDrive/WebDAV/template/139Cloud/189Cloud）。
package drivers

import (
	_ "xmedia/drivers/115_Open"
	_ "xmedia/drivers/123_Open"
	_ "xmedia/drivers/Baidu_Open"
	_ "xmedia/drivers/Guangya"
	_ "xmedia/drivers/LocalFs"
	_ "xmedia/drivers/Quark"
)
