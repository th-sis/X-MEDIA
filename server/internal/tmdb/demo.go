package tmdb

// demoItem 演示目录条目（无 TMDB Key 时兜底，保证开箱即测）。
type demoItem struct {
	ExternalID  int64
	Source      string
	MediaType   string
	Title       string
	TitleOrig   string
	Year        int
	VoteAvg     float64
	Overview    string
	Genres      []string
	Runtime     int
	Seasons     int
	EpisodeCnt  int
}

// demoCatalog 演示元数据目录。海报 URL 留空，由客户端渲染渐变占位海报。
var demoCatalog = []demoItem{
	{ExternalID: 19995, Source: "tmdb", MediaType: "movie", Title: "阿凡达", TitleOrig: "Avatar", Year: 2009, VoteAvg: 7.5, Runtime: 162, Genres: []string{"动作", "科幻", "冒险"}, Overview: "下身瘫痪的退伍士兵杰克·萨利被派往潘多拉星球，加入阿凡达计划，在人类与纳美人的冲突中做出抉择。"},
	{ExternalID: 27205, Source: "tmdb", MediaType: "movie", Title: "盗梦空间", TitleOrig: "Inception", Year: 2010, VoteAvg: 8.4, Runtime: 148, Genres: []string{"科幻", "动作", "惊悚"}, Overview: "一位技艺高超的窃贼能进入他人梦境窃取机密，这一次他接到一个看似不可能的任务：植入一个想法。"},
	{ExternalID: 550, Source: "tmdb", MediaType: "movie", Title: "搏击俱乐部", TitleOrig: "Fight Club", Year: 1999, VoteAvg: 8.4, Runtime: 139, Genres: []string{"剧情"}, Overview: "一个失眠的上班族与肥皂商人共同创办了一个地下搏击俱乐部，事情逐渐失控。"},
	{ExternalID: 155, Source: "tmdb", MediaType: "movie", Title: "蝙蝠侠：黑暗骑士", TitleOrig: "The Dark Knight", Year: 2008, VoteAvg: 8.5, Runtime: 152, Genres: []string{"动作", "犯罪", "剧情"}, Overview: "蝙蝠侠在戈登警长和检察官哈维·登特的协助下，对抗哥谭市新出现的犯罪天才小丑。"},
	{ExternalID: 24428, Source: "tmdb", MediaType: "movie", Title: "复仇者联盟", TitleOrig: "The Avengers", Year: 2012, VoteAvg: 7.7, Runtime: 143, Genres: []string{"科幻", "动作", "冒险"}, Overview: "神盾局集结各路超级英雄，组成复仇者联盟，共同对抗来自外星的威胁。"},
	{ExternalID: 157336, Source: "tmdb", MediaType: "movie", Title: "星际穿越", TitleOrig: "Interstellar", Year: 2014, VoteAvg: 8.4, Runtime: 169, Genres: []string{"科幻", "剧情", "冒险"}, Overview: "地球濒临毁灭，一队探险家穿越虫洞，为人类寻找新的家园。"},
	{ExternalID: 603, Source: "tmdb", MediaType: "movie", Title: "黑客帝国", TitleOrig: "The Matrix", Year: 1999, VoteAvg: 8.2, Runtime: 136, Genres: []string{"科幻", "动作"}, Overview: "一名黑客发现现实世界其实是由机器构建的虚拟矩阵，他必须选择红药丸或蓝药丸。"},
	{ExternalID: 122, Source: "tmdb", MediaType: "movie", Title: "指环王：王者归来", TitleOrig: "The Lord of the Rings: The Return of the King", Year: 2003, VoteAvg: 8.5, Runtime: 201, Genres: []string{"奇幻", "冒险", "动作"}, Overview: "魔戒远征队最后的征程，弗罗多与山姆继续向末日火山前进，中土世界的命运悬于一线。"},
	{ExternalID: 680, Source: "tmdb", MediaType: "movie", Title: "低俗小说", TitleOrig: "Pulp Fiction", Year: 1994, VoteAvg: 8.5, Runtime: 154, Genres: []string{"犯罪", "剧情"}, Overview: "几个相互交织的故事，讲述洛杉矶黑帮世界中的暴力与救赎。"},
	{ExternalID: 238, Source: "tmdb", MediaType: "movie", Title: "教父", TitleOrig: "The Godfather", Year: 1972, VoteAvg: 8.7, Runtime: 175, Genres: []string{"犯罪", "剧情"}, Overview: "科莱昂家族的兴衰史，以及小儿子迈克尔从局外人到家族掌权者的转变。"},
	{ExternalID: 13, Source: "tmdb", MediaType: "movie", Title: "阿甘正传", TitleOrig: "Forrest Gump", Year: 1994, VoteAvg: 8.5, Runtime: 142, Genres: []string{"剧情", "爱情"}, Overview: "智商只有75的阿甘，凭借单纯与执着，见证并参与了美国几十年的重大历史事件。"},
	{ExternalID: 278, Source: "tmdb", MediaType: "movie", Title: "肖申克的救赎", TitleOrig: "The Shawshank Redemption", Year: 1994, VoteAvg: 8.7, Runtime: 142, Genres: []string{"剧情"}, Overview: "银行家安迪被冤判入狱，在肖申克监狱中他与狱友瑞德结下深厚友谊，并策划了一场惊天越狱。"},
	{ExternalID: 14160, Source: "tmdb", MediaType: "movie", Title: "飞屋环游记", TitleOrig: "Up", Year: 2009, VoteAvg: 8.0, Runtime: 96, Genres: []string{"动画", "家庭", "冒险"}, Overview: "年迈的卡尔用气球带着房子飞向南美，意外带上了一名小童子军罗素，开启了一段奇妙的冒险。"},
	{ExternalID: 10681, Source: "tmdb", MediaType: "movie", Title: "机器人总动员", TitleOrig: "WALL·E", Year: 2008, VoteAvg: 8.0, Runtime: 98, Genres: []string{"动画", "科幻", "家庭"}, Overview: "地球上最后一个清扫机器人瓦力，在遇见探员伊娃后，展开了一段跨越银河的冒险。"},
	{ExternalID: 87101, Source: "tmdb", MediaType: "movie", Title: "火星救援", TitleOrig: "The Martian", Year: 2015, VoteAvg: 7.7, Runtime: 141, Genres: []string{"科幻", "剧情", "冒险"}, Overview: "宇航员马克·沃特尼被遗留在火星上，他必须依靠自己的智慧在恶劣环境中求生。"},

	{ExternalID: 1399, Source: "tmdb", MediaType: "tv", Title: "权力的游戏", TitleOrig: "Game of Thrones", Year: 2011, VoteAvg: 8.4, Seasons: 8, EpisodeCnt: 73, Genres: []string{"剧情", "奇幻"}, Overview: "维斯特洛大陆的几大家族为争夺铁王座展开权谋斗争，而北境之外，古老的威胁正在逼近。"},
	{ExternalID: 1396, Source: "tmdb", MediaType: "tv", Title: "绝命毒师", TitleOrig: "Breaking Bad", Year: 2008, VoteAvg: 8.6, Seasons: 5, EpisodeCnt: 62, Genres: []string{"剧情", "犯罪"}, Overview: "一名高中化学老师在得知自己身患绝症后，走上制毒之路，只为给家人留下生活保障。"},
	{ExternalID: 66732, Source: "tmdb", MediaType: "tv", Title: "怪奇物语", TitleOrig: "Stranger Things", Year: 2016, VoteAvg: 8.6, Seasons: 4, EpisodeCnt: 34, Genres: []string{"科幻", "悬疑", "剧情"}, Overview: "印第安纳州霍金斯小镇上，一名男孩神秘失踪，一群朋友在寻找他的过程中揭开了超自然力量的秘密。"},
	{ExternalID: 82856, Source: "tmdb", MediaType: "tv", Title: "曼达洛人", TitleOrig: "The Mandalorian", Year: 2019, VoteAvg: 8.5, Seasons: 3, EpisodeCnt: 24, Genres: []string{"科幻", "动作", "冒险"}, Overview: "一名孤独的赏金猎人在银河系边缘执行任务，却意外保护起一个神秘的孩子。"},
	{ExternalID: 94997, Source: "tmdb", MediaType: "tv", Title: "龙之家族", TitleOrig: "House of the Dragon", Year: 2022, VoteAvg: 8.4, Seasons: 2, EpisodeCnt: 18, Genres: []string{"剧情", "奇幻"}, Overview: "坦格利安家族内部的权力斗争，最终导致了一场名为血龙狂舞的内战。"},
	{ExternalID: 1402, Source: "tmdb", MediaType: "tv", Title: "行尸走肉", TitleOrig: "The Walking Dead", Year: 2010, VoteAvg: 8.1, Seasons: 11, EpisodeCnt: 177, Genres: []string{"剧情", "恐怖", "惊悚"}, Overview: "丧尸病毒爆发后的世界，一群幸存者在废墟中挣扎求生。"},
	{ExternalID: 63247, Source: "tmdb", MediaType: "tv", Title: "西部世界", TitleOrig: "Westworld", Year: 2016, VoteAvg: 8.0, Seasons: 4, EpisodeCnt: 36, Genres: []string{"科幻", "西部", "剧情"}, Overview: "在一个由人工智能接待员构成的西部主题乐园里，游客可以尽情放纵，但接待员们开始觉醒。"},
	{ExternalID: 8592, Source: "tmdb", MediaType: "tv", Title: "老友记", TitleOrig: "Friends", Year: 1994, VoteAvg: 8.4, Seasons: 10, EpisodeCnt: 236, Genres: []string{"喜剧", "剧情"}, Overview: "六位住在纽约的好友，在彼此的陪伴下走过人生中的喜怒哀乐。"},
	{ExternalID: 456, Source: "tmdb", MediaType: "tv", Title: "辛普森一家", TitleOrig: "The Simpsons", Year: 1989, VoteAvg: 8.0, Seasons: 35, EpisodeCnt: 768, Genres: []string{"动画", "喜剧"}, Overview: "春田镇辛普森一家的日常生活，一部经久不衰的动画情景喜剧。"},

	{ExternalID: 37854, Source: "tmdb", MediaType: "tv", Title: "进击的巨人", TitleOrig: "Shingeki no Kyojin", Year: 2013, VoteAvg: 8.6, Seasons: 4, EpisodeCnt: 87, Genres: []string{"动画", "动作", "奇幻"}, Overview: "人类被巨人逼入高墙之内，少年艾伦发誓要消灭所有巨人，夺回自由。"},
	{ExternalID: 31911, Source: "tmdb", MediaType: "tv", Title: "钢之炼金术师", TitleOrig: "Fullmetal Alchemist: Brotherhood", Year: 2009, VoteAvg: 8.7, Seasons: 1, EpisodeCnt: 64, Genres: []string{"动画", "冒险", "奇幻"}, Overview: "两兄弟为复活母亲进行禁忌的人体炼成，代价惨重，他们踏上了寻找贤者之石的旅程。"},
	{ExternalID: 209867, Source: "tmdb", MediaType: "tv", Title: "鬼灭之刃", TitleOrig: "Kimetsu no Yaiba", Year: 2019, VoteAvg: 8.6, Seasons: 4, EpisodeCnt: 55, Genres: []string{"动画", "动作", "奇幻"}, Overview: "家人惨遭鬼杀害的炭治郎，为让变成鬼的妹妹变回人类，加入了猎鬼人的行列。"},
	{ExternalID: 1104, Source: "tmdb", MediaType: "tv", Title: "攻壳机动队", TitleOrig: "Ghost in the Shell", Year: 2002, VoteAvg: 8.0, Seasons: 2, EpisodeCnt: 52, Genres: []string{"动画", "科幻"}, Overview: "在高度信息化的未来，公安九课与各种网络犯罪作斗争，探讨意识与存在的边界。"},

	{ExternalID: 50563, Source: "tmdb", MediaType: "documentary", Title: "蓝色星球", TitleOrig: "The Blue Planet", Year: 2001, VoteAvg: 8.9, Seasons: 2, EpisodeCnt: 15, Genres: []string{"纪录片"}, Overview: "BBC 制作的海洋纪录片，探索地球上最广阔的未知世界。"},
	{ExternalID: 48107, Source: "tmdb", MediaType: "documentary", Title: "地球脉动", TitleOrig: "Planet Earth", Year: 2006, VoteAvg: 8.9, Seasons: 3, EpisodeCnt: 22, Genres: []string{"纪录片"}, Overview: "从两极到赤道，从高山到深海，展现地球生物的多样与壮美。"},

	{ExternalID: 90203, Source: "tmdb", MediaType: "variety", Title: "中国好声音", TitleOrig: "The Voice of China", Year: 2012, VoteAvg: 6.8, Seasons: 12, EpisodeCnt: 120, Genres: []string{"综艺", "音乐"}, Overview: "大型音乐选秀节目，导师盲选学员，用声音打动人心。"},
}
