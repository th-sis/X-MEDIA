// Package drivers 通过空导入聚合所有驱动，触发各驱动的 init() 注册。
package drivers

import (
	_ "xmedia/drivers/115_Open"
	_ "xmedia/drivers/123_Open"
	_ "xmedia/drivers/139Cloud"
	_ "xmedia/drivers/189Cloud"
	_ "xmedia/drivers/Baidu_Open"
	_ "xmedia/drivers/Guangya"
	_ "xmedia/drivers/LocalFs"
	_ "xmedia/drivers/OneDrive"
	_ "xmedia/drivers/Quark"
	_ "xmedia/drivers/WebDAV"
)
