package i18n

// CurseForge class names, as the API returns them, in Chinese.
func init() {
	for k, v := range map[string]string{
		"armor, tools, and weapons": "装备、工具与武器", "adventure and rpg": "冒险与角色扮演", "technology": "科技", "magic": "魔法",
		"storage": "存储", "api and library": "前置库", "utility & qol": "实用与便利", "map and information": "地图与信息",
		"cosmetic": "装饰", "world gen": "世界生成", "biomes": "生物群系", "ores and resources": "矿石与资源",
		"structures": "结构", "dimensions": "维度", "mobs": "生物", "food": "食物", "redstone": "红石",
		"player transport": "交通", "energy": "能源", "processing": "加工", "genetics": "遗传", "automation": "自动化",
		"farming": "农业", "server utility": "服务器工具", "miscellaneous": "杂项", "bug fixes": "问题修复",
		"education": "教育", "addons": "附属", "twitch integration": "直播互动", "skyblock": "空岛", "performance": "性能",
		"energy, fluid, and item transport": "物流", "item fluid and energy transport": "物流", "mcreator": "MCreator",
		"create": "机械动力", "kubejs": "KubeJS", "crafttweaker": "CraftTweaker", "applied energistics 2": "应用能源 2",
		"thermal expansion": "热力膨胀", "tinker's construct": "匠魂", "industrial craft": "工业时代", "blood magic": "血魔法",
		"thaumcraft": "神秘时代", "integrated dynamics": "集成动力", "galacticraft": "星系", "buildcraft": "建筑",
		"forestry": "林业", "configuration": "配置", "library": "前置库", "utility": "实用",
		"exploration": "探索", "hardcore": "硬核", "quests": "任务", "multiplayer": "多人", "small / light": "轻量",
		"extra large": "大型", "sci-fi": "科幻", "tech": "科技", "combat / pvp": "战斗", "map based": "地图",
		"16x": "16x", "32x": "32x", "64x": "64x", "128x": "128x", "256x": "256x", "512x and higher": "512x+",
		"animated": "动态", "medieval": "中世纪", "modern": "现代", "photo realistic": "写实", "steampunk": "蒸汽朋克",
		"traditional": "传统", "mod support": "模组支持", "data packs": "数据包", "font packs": "字体",
	} {
		if _, ok := zhCategories[k]; !ok {
			zhCategories[k] = v
		}
	}
}
