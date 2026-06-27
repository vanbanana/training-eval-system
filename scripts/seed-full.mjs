#!/usr/bin/env node
/**
 * 完整模拟数据种子 v2 — 使用 node:sqlite 原生绑定
 *
 * 生成: 3 教师 + 180 学生 + 6 课程 + 30 班级 + 14 任务 + 完整评价/提交/通知/审计日志/相似度/学生画像
 *
 * 前置: go run ./cmd/seed (确保 admin/teacher1/student1 存在)
 * 用法: node scripts/seed-full.mjs
 */
import { DatabaseSync } from 'node:sqlite'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const DB_PATH = path.resolve(__dirname, '../go-backend/data/app.db')
const BCRYPT_HASH = '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2' // = test123

console.time('seed-total')

// ─── DB Init ──────────────────────────────────────────────────
const db = new DatabaseSync(DB_PATH)
db.exec('PRAGMA journal_mode=WAL')
db.exec('PRAGMA foreign_keys=OFF')

function run(sql, ...params) {
  try { return db.prepare(sql).run(...params) } catch (e) { return null }
}
function get(sql, ...params) {
  try {
    const rows = db.prepare(sql).all(...params)
    return rows.length > 0 ? rows[0] : null
  } catch { return null }
}
function all(sql, ...params) {
  try { return db.prepare(sql).all(...params) } catch { return [] }
}

// ─── Helpers ──────────────────────────────────────────────────
const NOW = new Date()
function daysAgo(d) {
  const t = new Date(NOW.getTime() - d * 86400000)
  return t.toISOString().replace('T', ' ').substring(0, 19)
}
function daysFromNow(d) {
  const t = new Date(NOW.getTime() + d * 86400000)
  return t.toISOString().replace('T', ' ').substring(0, 19)
}
const rand = (min, max) => Math.floor(Math.random() * (max - min + 1)) + min
const randf = (min, max, dec = 1) => {
  const v = min + Math.random() * (max - min)
  return Math.round(v * Math.pow(10, dec)) / Math.pow(10, dec)
}
const pick = (arr) => arr[Math.floor(Math.random() * arr.length)]
const pickN = (arr, n) => {
  const shuffled = [...arr].sort(() => Math.random() - 0.5)
  return shuffled.slice(0, Math.min(n, arr.length))
}
const clamp = (v, min, max) => Math.max(min, Math.min(max, v))

// ─── Data Definitions ──────────────────────────────────────────
const TEACHERS = [
  { username: 'teacher2', display_name: '王教授' },
  { username: 'teacher3', display_name: '李副教授' },
  { username: 'teacher4', display_name: '陈讲师' },
]

const COURSES = [
  { name: '计算机网络', code: 'CS-NET', teacher_idx: 0 },
  { name: '操作系统原理', code: 'CS-OS', teacher_idx: 1 },
  { name: '数据库系统', code: 'CS-DB', teacher_idx: 2 },
  { name: '软件工程', code: 'CS-SE', teacher_idx: 0 },
  { name: '算法设计与分析', code: 'CS-ALGO', teacher_idx: 1 },
  { name: 'Python程序设计', code: 'CS-PY', teacher_idx: 2 },
]

const CLASS_NAMES = [
  '计科2101班','计科2102班','计科2103班','计科2104班','计科2105班',
  '软工2101班','软工2102班','软工2103班','软工2104班','软工2105班',
  '网络2101班','网络2102班','网络2103班',
  '大数据2101班','大数据2102班',
  '人工智能2101班','人工智能2102班',
  '信安2101班','信安2102班','信安2103班',
  '物联网2101班','物联网2102班',
  '数媒2101班','数媒2102班',
  '嵌入式2101班','嵌入式2102班',
  '云计算2101班','云计算2102班',
  '智科2101班','智科2102班',
]

const SURNAMES = '王李张刘陈杨黄赵周吴徐孙马朱胡郭何高林罗郑梁谢宋唐韩曹许邓冯程蔡彭潘袁董余苏叶卢蒋田杜丁沈任姚傅钟魏'.split('')
const GIVEN_MALE = '伟强磊军勇杰涛明超浩志远泽宇天宇俊杰浩然子涵宇航嘉豪明轩泽洋博文昊天子轩文博思远泽楷'.split('')
const GIVEN_FEMALE = '芳娟敏静丽婷雪琳萍红思琪雨桐诗涵雅琪欣怡晓彤悦彤梦瑶雨萱思涵紫涵慧敏佳琪心怡梓涵婷玉'.split('')
function randomName(i) {
  const s = SURNAMES[i % SURNAMES.length]
  const g = i % 3 === 0 ? GIVEN_FEMALE[i % GIVEN_FEMALE.length] : GIVEN_MALE[i % GIVEN_MALE.length]
  return s + g
}

// Generate 180 students
const STUDENT_NAMES = []
for (let i = 2; i <= 181; i++) STUDENT_NAMES.push(randomName(i))

const TASKS = [
  // 教师1 (王教授) — 计算机网络、软件工程
  { name: 'TCP/IP协议分析实验', description: '使用Wireshark抓包分析TCP/IP协议栈，理解三次握手与四次挥手', requirements: '1. 抓包文件 .pcapng\n2. 协议分析报告 PDF\n3. 各层头部结构图解', eval_criteria: '抓包完整性、分析深度、报告规范性', course_idx: 0, status: 'published', deadline_days: 7, teacher_idx: 0, dims: [{name:'抓包完整性',w:30},{name:'分析深度',w:35},{name:'报告规范性',w:20},{name:'抓包技巧',w:15}] },
  { name: '子网划分与路由配置', description: '设计并配置VLSM子网划分方案，配置静态路由与默认路由', requirements: '1. 子网划分方案表\n2. 路由配置截图\n3. 连通性测试报告', eval_criteria: '方案合理性、配置正确性、测试完整性', course_idx: 0, status: 'published', deadline_days: 14, teacher_idx: 0, dims: [{name:'方案设计',w:30},{name:'配置正确性',w:35},{name:'测试验证',w:20},{name:'文档质量',w:15}] },
  { name: '网络安全协议分析', description: '分析HTTPS/TLS握手过程，对比HTTP与HTTPS的安全性差异', requirements: '1. 抓包分析截图\n2. 安全对比报告\n3. 证书链分析', eval_criteria: '分析深度、理解程度、报告质量', course_idx: 0, status: 'published', deadline_days: 10, teacher_idx: 0, dims: [{name:'TLS分析',w:35},{name:'安全性对比',w:30},{name:'证书分析',w:20},{name:'报告质量',w:15}] },
  { name: '网络应用开发实训', description: '基于Socket编程实现一个简单的聊天室应用', requirements: '1. 源代码\n2. 程序运行截图\n3. 设计文档', eval_criteria: '功能完整性、代码质量、文档质量', course_idx: 0, status: 'published', deadline_days: 21, teacher_idx: 0, dims: [{name:'功能完整性',w:35},{name:'代码质量',w:25},{name:'并发处理',w:25},{name:'设计文档',w:15}] },
  { name: '需求分析文档撰写', description: '针对选定的软件项目撰写完整的需求规格说明书', requirements: '1. SRS文档\n2. 用例图\n3. 原型设计截图', eval_criteria: '需求完整性、文档规范性、图表质量', course_idx: 3, status: 'published', deadline_days: 5, teacher_idx: 0, dims: [{name:'需求完整性',w:35},{name:'文档规范性',w:25},{name:'用例建模',w:25},{name:'原型设计',w:15}] },
  { name: '软件设计模式实践', description: '在实际场景中应用至少3种设计模式进行系统设计', requirements: '1. 设计文档\n2. 类图/时序图\n3. 代码实现', eval_criteria: '模式选择合理性、实现正确性、文档质量', course_idx: 3, status: 'closed', deadline_days: -5, teacher_idx: 0, dims: [{name:'模式应用',w:35},{name:'实现正确性',w:30},{name:'UML建模',w:20},{name:'设计说明',w:15}] },
  { name: '敏捷开发与Scrum实践', description: '采用Scrum框架完成一个小型项目的迭代开发', requirements: '1. Sprint计划\n2. 看板截图\n3. 迭代回顾报告', eval_criteria: '流程规范性、团队协作、交付质量', course_idx: 3, status: 'draft', deadline_days: null, teacher_idx: 0, dims: [{name:'流程规范性',w:30},{name:'迭代管理',w:25},{name:'交付质量',w:30},{name:'回顾总结',w:15}] },
  // 教师2 (李副教授) — 操作系统原理、算法设计与分析
  { name: '进程调度模拟实验', description: '模拟FCFS、SJF、RR三种调度算法并对比性能指标', requirements: '1. 模拟程序源码\n2. 甘特图\n3. 性能对比报告', eval_criteria: '算法实现正确性、性能对比完整性、可视化展示', course_idx: 1, status: 'published', deadline_days: 10, teacher_idx: 1, dims: [{name:'算法实现',w:35},{name:'性能对比',w:30},{name:'可视化展示',w:20},{name:'报告质量',w:15}] },
  { name: '内存管理页面置换', description: '模拟FIFO、LRU、OPT三种页面置换算法并分析缺页率', requirements: '1. 模拟程序源码\n2. 缺页率图表\n3. 分析报告', eval_criteria: '算法正确性、数据分析、报告质量', course_idx: 1, status: 'published', deadline_days: 8, teacher_idx: 1, dims: [{name:'算法正确性',w:35},{name:'数据分析',w:30},{name:'代码质量',w:20},{name:'实验报告',w:15}] },
  { name: '文件系统设计实现', description: '设计并实现一个简单的文件系统，支持基本文件操作', requirements: '1. 文件系统源码\n2. 设计文档\n3. 测试用例', eval_criteria: '功能完整性、设计合理性、代码质量', course_idx: 1, status: 'closed', deadline_days: -3, teacher_idx: 1, dims: [{name:'功能完整性',w:35},{name:'设计合理性',w:25},{name:'代码质量',w:20},{name:'测试覆盖',w:20}] },
  { name: '排序算法性能基准测试', description: '实现快排、归并、堆排序并进行大规模数据基准测试', requirements: '1. 源代码\n2. 基准测试结果 CSV\n3. 性能分析报告', eval_criteria: '实现正确性、测试方法、分析深度', course_idx: 4, status: 'published', deadline_days: 12, teacher_idx: 1, dims: [{name:'实现正确性',w:30},{name:'性能测试',w:30},{name:'复杂度分析',w:25},{name:'分析报告',w:15}] },
  { name: '图算法应用实训', description: '实现最短路径(Dijkstra)和最小生成树(Kruskal/Prim)算法', requirements: '1. 算法源码\n2. 测试数据集\n3. 应用案例报告', eval_criteria: '算法正确性、代码质量、应用分析', course_idx: 4, status: 'draft', deadline_days: null, teacher_idx: 1, dims: [{name:'算法正确性',w:35},{name:'代码质量',w:20},{name:'应用分析',w:30},{name:'测试验证',w:15}] },
  // 教师3 (陈讲师) — 数据库系统、Python程序设计
  { name: 'SQL查询优化实验', description: '对给定的数据库表进行SQL查询优化，分析执行计划', requirements: '1. 原始SQL与优化SQL\n2. 执行计划截图\n3. 优化分析报告', eval_criteria: '优化效果、执行计划分析、报告完整性', course_idx: 2, status: 'published', deadline_days: 6, teacher_idx: 2, dims: [{name:'优化效果',w:35},{name:'执行计划分析',w:30},{name:'索引设计',w:20},{name:'报告完整性',w:15}] },
  { name: '数据库设计实训', description: '设计一个图书管理系统的数据库，包含ER图和范式分析', requirements: '1. ER图\n2. 建表SQL\n3. 范式分析文档', eval_criteria: '设计规范性、完整性、文档质量', course_idx: 2, status: 'published', deadline_days: 15, teacher_idx: 2, dims: [{name:'ER设计',w:30},{name:'规范化程度',w:30},{name:'SQL实现',w:25},{name:'设计文档',w:15}] },
  { name: 'Python爬虫与数据清洗', description: '编写网络爬虫抓取数据并进行清洗和分析', requirements: '1. 爬虫源码\n2. 抓取数据样本\n3. 数据清洗报告', eval_criteria: '爬虫功能、数据质量、代码规范', course_idx: 5, status: 'published', deadline_days: 10, teacher_idx: 2, dims: [{name:'爬虫功能',w:30},{name:'数据质量',w:25},{name:'代码规范',w:20},{name:'反爬处理',w:15},{name:'分析报告',w:10}] },
  { name: '数据分析与可视化', description: '使用Pandas和Matplotlib完成数据分析与可视化报告', requirements: '1. 分析代码\n2. 可视化图表\n3. 分析报告', eval_criteria: '分析方法、可视化效果、报告质量', course_idx: 5, status: 'closed', deadline_days: -8, teacher_idx: 2, dims: [{name:'数据预处理',w:25},{name:'分析方法',w:30},{name:'可视化效果',w:25},{name:'分析报告',w:20}] },
]

// Per-dimension critique pool
const CRITIQUES = {
  '抓包完整性': ['抓包覆盖了完整的三次握手和四次挥手', '缺少部分关键包的抓取', '抓包过程完整，过滤规则使用恰当', '抓包数据充分，分析了主要协议交互', '部分包的细节未展开分析'],
  '分析深度': ['分析深入，逐字段解读了协议头部', '分析比较全面但缺少量化对比', '分析停留在表面，深度不足', '有独到的协议行为分析见解', '分析层次清晰，论据充分'],
  '报告规范性': ['报告结构完整，排版优美，图表丰富', '报告格式规范但缺少部分关键图表', '报告内容充实但结构略显混乱', '报告规范性好，符合学术写作标准', '报告过于简略，需要补充详细内容'],
  '方案设计': ['子网划分方案合理，IP利用率高', '方案设计可以，但存在优化空间', '设计思路清晰，考虑了扩展性', '方案过于保守，IP地址浪费较多', '方案设计完整，VLSM计算正确'],
  '配置正确性': ['路由配置完全正确，网络全连通', '基本配置正确，有一处路由遗漏', '配置无误，连通性测试全部通过', '配置有误，部分网络不可达', '配置方案合理，实施了冗余备份'],
  '测试验证': ['测试用例充分覆盖了所有场景', '连通性测试完整，验证方法科学', '测试覆盖率有待提高', '测试方法正确，结果数据详实', '缺少边界条件测试'],
  '文档质量': ['文档齐全，图文并茂，格式规范', '文档结构清晰但缺少部分技术细节', '文档编写规范，有参考价值', '文档过于简单，需要补充详细说明', '文档完整，包含所有必要部分'],
  'TLS分析': ['TLS握手过程分析完整，理解了加密套件协商', '分析了证书链验证过程，理解深入', 'TLS分析较完整但缺少协议版本对比', '对TLS 1.3新特性有较好分析', 'TLS握手分析停留在抓包层面'],
  '安全性对比': ['HTTP/HTTPS安全性对比全面，理解深刻', '对比维度完善，结合实际场景分析', '安全性对比基本完整，深度有待提高', '对比分析深入，包含中间人攻击原理', '安全性差异分析停留在表面'],
  '证书分析': ['证书链分析完整，理解了CA信任体系', '证书解析正确，理解了扩展字段含义', '证书分析较详细但缺少吊销验证', '对自签名证书和CA证书做了对比', '证书分析基本完整'],
  '功能完整性': ['功能实现完整，覆盖所有需求且处理了异常', '核心功能完善，扩展功能有待补充', '所有功能正确实现，用例充分', '基本功能实现，缺少边界处理', '功能完整，用户体验良好'],
  '代码质量': ['代码整洁规范，模块化程度高，注释充分', '代码结构合理，可维护性强', '代码风格一般，有改进空间', '代码冗余较多，建议重构', '代码质量优秀，遵循了编码规范'],
  '并发处理': ['并发控制正确，使用锁机制恰当', '多线程同步正确，无竞态条件', '并发处理基本正确，性能有待优化', '并发模型设计合理，扩展性好', '存在潜在的线程安全问题'],
  '设计文档': ['设计文档详尽，包含架构图和接口说明', '文档完整但缺少部分详细设计', '设计说明清晰，技术选型合理', '文档结构规范但缺少时序图', '设计文档涵盖全面，有参考价值'],
  '需求完整性': ['功能性和非功能性需求定义完整清晰', '需求覆盖了主要功能但缺少非功能性需求', '需求分析全面，用例覆盖了所有场景', '需求规格完整，符合IEEE标准', '部分需求定义模糊需要澄清'],
  '文档规范性': ['文档完全符合规范标准，结构严谨', '文档结构规范但缺少术语表', '文档编写遵循了标准模板', '规范性好但部分章节组织可优化', '文档基本规范'],
  '用例建模': ['用例图准确完整，包含了扩展用例', '用例建模合理，覆盖了主要功能', '用例图清晰但缺少泛化关系', '用例识别准确，参与者定义正确', '用例建模完整，场景描述详细'],
  '原型设计': ['原型设计完整，交互流畅，可用性好', '原型覆盖了核心功能流程', '原型设计尚可但视觉设计有待提升', '原型交互设计合理，用户路径清晰', '原型完整但缺少移动端适配'],
  '模式应用': ['设计模式应用恰当，解决了实际问题', '模式选择合理但实现略有偏差', '至少三种模式正确实现并说明了选择理由', '模式理解深入，应用场景匹配', '模式应用基本正确'],
  'UML建模': ['类图和时序图规范准确，关系清晰', 'UML图完整但缺少状态图', '建模准确，遵循了UML规范', '类图设计合理但缺少关键关系', 'UML图覆盖了核心设计'],
  '设计说明': ['设计思路阐述清晰，决策理由充分', '设计说明完整但部分决策缺少论证', '技术方案对比分析做得好', '设计说明条理清晰，易于理解', '设计说明基本完整'],
  '流程规范性': ['Scrum流程完全按规范执行，包含所有环节', '流程基本规范但部分环节执行不到位', 'Sprint规划合理，每日站会坚持执行', '流程规范性好，产出了完整工件', '流程执行有偏差需要调整'],
  '迭代管理': ['Sprint计划和跟踪完整，燃尽图准确', '迭代管理规范，任务分解合理', 'Sprint回顾有深度，改进了流程', '迭代管理基本到位但估算偏差较大', '迭代管理数据完整'],
  '交付质量': ['迭代交付物质量高，通过了所有验收标准', '交付质量稳定，Bug率控制在合理范围', '交付物基本达标但部分功能有待完善', '交付质量好，用户反馈积极', '交付物完整通过验收'],
  '回顾总结': ['迭代回顾深刻，提出了可执行改进方案', '回顾内容充实但改进项追踪不充分', '回顾会议开诚布公，改进落地好', '回顾总结全面分析了流程数据', '回顾质量好'],
  '算法实现': ['三种调度算法完全正确实现，代码高效', '算法实现正确但性能有待优化', '实现完整，包含了公平性分析', '算法实现基本正确，边界处理完善', '实现质量好，输出结果准确'],
  '性能对比': ['性能对比维度全面，数据可视化呈现好', '对比分析详实，指标选取合理', '性能数据分析深入，结论有说服力', '性能对比完整但缺少大规模数据测试', '对比数据充分'],
  '可视化展示': ['可视化图表美观清晰，有效传达了数据', '可视化丰富但部分图表选择不当', '甘特图等展示完整，直观易懂', '可视化设计好，交互式展示加分', '可视化展示基本完整'],
  '报告质量': ['实验报告质量高，结构完整，分析深入', '报告内容充实但排版可进一步优化', '报告格式规范数据详实', '报告质量和分析深度都很好', '报告基本完整但缺少关键分析'],
  '算法正确性': ['算法实现完全正确，通过了所有测试用例', '核心算法正确，边界情况有误', '实现完整，时间空间复杂度满足要求', '算法思路清晰，实现准确高效', '基本正确但优化空间较大'],
  '数据分析': ['数据分析全面，缺页率对比深入', '数据分析方法正确，结论合理', '数据统计完整但缺少统计显著性检验', '分析维度全面，图表辅助表达好', '数据分析基本正确'],
  '实验报告': ['实验报告详细完整，包含实验过程和结果分析', '报告结构规范但缺少实验反思', '报告数据详实，图表清晰', '报告完整覆盖了所有实验要求', '报告基本符合要求'],
  '测试覆盖': ['测试用例充分，覆盖了正常和异常路径', '测试覆盖了主要功能但缺少边界测试', '测试方法科学，包含单元和集成测试', '测试覆盖率达标但缺少性能测试', '测试数据完整可用'],
  '性能测试': ['性能测试方法科学，数据具有统计意义', '基准测试完整，多种数据规模对比好', '性能数据分析深入，发现了性能瓶颈', '测试方法正确但缺少环境说明', '性能数据充分'],
  '复杂度分析': ['时间空间复杂度分析准确深入', '复杂度分析正确但有优化空间', '分析了最好最坏平均情况，理解透彻', '复杂度分析完整包含了推导过程', '复杂度分析基本正确'],
  '应用分析': ['结合实际场景分析深入，案例具体', '应用分析完整但缺少对比方案', '应用场景分析合理，考虑因素全面', '分析有独到见解，对实际问题有指导意义', '应用分析基本完整'],
  '测试验证': ['测试数据充分，验证了算法在不同规模下的表现', '验证方法科学，结果可信', '测试数据完整但缺少压力测试', '验证过程详细，可复现性好', '测试验证基本完成'],
  '优化效果': ['SQL性能提升显著，执行时间减少了60%+', '优化有成效但效果不够明显', '优化策略正确，执行计划分析到位', '多种优化方法对比好，选择了最佳方案', '优化效果数据详实'],
  '执行计划分析': ['对EXPLAIN输出解读深入，理解了查询执行过程', '执行计划分析正确但缺少索引使用分析', '分析全面对比了优化前后的执行计划', '分析深入发现了瓶颈所在', '执行计划分析基本完整'],
  '索引设计': ['索引设计合理，覆盖了主要查询模式', '索引策略正确但有冗余索引', '索引设计考虑了查询和写入平衡', '索引设计完整包含复合索引', '索引设计基本合理'],
  '报告完整性': ['报告完整包含所有实验要求和扩展内容', '报告结构完整但缺少部分实验截图', '报告内容全面，数据支撑充分', '报告完整涵盖了所有必做和选做内容', '报告基本完整'],
  'ER设计': ['ER图设计规范，实体关系表达清晰准确', 'ER设计完整但缺少部分弱实体', 'ER建模正确，转换范式规范', '设计合理包含了完整业务规则', 'ER设计基本完整'],
  '规范化程度': ['范式分解正确达到了3NF/BCNF标准', '规范化分析准确但部分依赖未识别', '范式分析完整，包含推导过程', '规范化程度好，消除了数据冗余', '范式分解基本正确'],
  'SQL实现': ['建表SQL完整，约束定义全面', 'SQL实现正确但缺少部分检查约束', '建表语句规范包含了索引定义', 'SQL实现完整含外键参照完整性', 'SQL编写正确'],
  '爬虫功能': ['爬虫能正确抓取目标网站的完整数据', '爬虫工作正常但速度有待优化', '爬虫功能完整处理了分页和动态加载', '爬虫数据抓取完整含反爬绕过', '爬虫功能基本可用'],
  '数据质量': ['数据清洗完整，格式统一，质量高', '数据预处理完善但部分异常值未处理', '数据质量好，缺失值处理得当', '清洗流程规范包含了数据验证', '数据质量基本达标'],
  '代码规范': ['代码风格一致遵循PEP8，异常处理完善', '代码规范但缺少类型注解', '代码可读性好包含完整docstring', '代码规范优秀模块化设计好', '代码基本符合规范'],
  '反爬处理': ['有效处理了多种反爬机制策略得当', '反爬策略基本有效但部分场景失效', '反爬方案完整包含了代理轮换', '反爬处理有一定效果但不够稳定', '反爬策略基本合理'],
  '数据预处理': ['数据预处理完整包含了数据清洗和特征工程', '预处理方法得当但部分异常值未处理', '数据清洗流程规范，处理了缺失值', '预处理全面包含了归一化和编码', '数据预处理基本完整'],
  '分析方法': ['数据分析方法选择恰当，结论可靠', '分析方法正确但统计检验不够规范', '分析流程清晰，可视化辅助理解', '分析深入多维对比好', '分析方法基本正确'],
  '可视化效果': ['可视化图表美观专业，有效传达数据故事', '图表设计好但配色可优化', '可视化多样包含交互式展示', '图表清晰准确标签完整', '可视化效果基本达到要求'],
}

function getCritique(dimName) {
  const pool = CRITIQUES[dimName]
  if (pool) return pick(pool)
  // Generic fallback
  const fallbacks = ['表现良好，达到要求', '完成质量较好，有改进空间', '基本完成，细节有待加强', '完成优秀，超出预期', '部分完成，需要补充']
  return pick(fallbacks)
}

// ─── Main ─────────────────────────────────────────────────────
function main() {
  console.log('╔══════════════════════════════════════════╗')
  console.log('║    完整模拟数据种子 v2                   ║')
  console.log('╚══════════════════════════════════════════╝\n')

  // Step 1: Check existing
  const userCount = get('SELECT COUNT(*) as c').c
  console.log(`[1/12] 当前用户数: ${userCount}`)

  // Step 2: Create teachers
  console.log('\n[2/12] 创建教师...')
  for (const t of TEACHERS) {
    run('INSERT OR IGNORE INTO users (username, display_name, password_hash, role, is_active) VALUES (?,?,?,?,1)',
      t.username, t.display_name, BCRYPT_HASH, 'teacher')
    const u = get(`SELECT id FROM users WHERE username=?`, t.username)
    console.log(`  ✓ ${t.username} / ${t.display_name} / test123 → id=${u.id}`)
  }

// Step 3: Create students
  console.log('\n[3/12] 创建 180 名学生...')
  let batchValues = []
  for (let i = 0; i < STUDENT_NAMES.length; i++) {
    const username = `student${i + 2}`
    batchValues.push(`('${username}', '${STUDENT_NAMES[i]}', '${BCRYPT_HASH}', 'student', 1)`)
    if (batchValues.length >= 50 || i === STUDENT_NAMES.length - 1) {
      db.exec(`INSERT OR IGNORE INTO users (username, display_name, password_hash, role, is_active) VALUES ${batchValues.join(',')}`)
      batchValues = []
    }
  }
  const stuCount = get("SELECT COUNT(*) as c FROM users WHERE role='student'").c
  console.log(`  ✓ 当前学生总数: ${stuCount}`)

  // Step 4: Get all IDs
  const teacherRows = all("SELECT id, username FROM users WHERE role='teacher' ORDER BY id")
  const teacherIds = teacherRows.map(r => r.id)
  const teacherMap = {}
  for (const t of teacherRows) {
    const idx = TEACHERS.findIndex(tc => tc.username === t.username)
    if (idx >= 0) teacherMap[idx] = t.id
  }
  const studentRows = all("SELECT id FROM users WHERE role='student' ORDER BY id")
  const studentIds = studentRows.map(r => r.id)
  const adminId = get("SELECT id FROM users WHERE role='admin' LIMIT 1").id

  // Step 5: Create courses
  console.log('\n[4/12] 创建 6 门课程...')
  const courseMap = {}
  for (const c of COURSES) {
    run('INSERT OR IGNORE INTO courses (name, code) VALUES (?,?)', c.name, c.code)
    const row = get('SELECT id FROM courses WHERE code=?', c.code)
    courseMap[c.name] = row.id
  }
  console.log(`  ✓ 课程数: ${Object.keys(courseMap).length}`)

  // Step 6: Create 30 classes
  console.log('\n[5/12] 创建 30 个班级...')
  const classMap = {}
  // Build teacher → courses map
  const teacherCourseMap = {}
  for (let ti = 0; ti < 3; ti++) {
    teacherCourseMap[ti] = COURSES.filter(c => c.teacher_idx === ti).map(c => c.name)
  }
  for (let i = 0; i < CLASS_NAMES.length; i++) {
    const name = CLASS_NAMES[i]
    const ti = i % 3
    const tc = teacherCourseMap[ti]
    const courseName = tc[i % tc.length]
    const tid = teacherMap[ti]
    run('INSERT OR IGNORE INTO classes (name, course_id, teacher_id, student_count) VALUES (?,?,?,6)',
      name, courseMap[courseName], tid)
    const row = get('SELECT id FROM classes WHERE name=?', name)
    classMap[name] = row.id
  }
  console.log(`  ✓ 班级数: ${Object.keys(classMap).length}`)

  // Step 7: Assign students to classes (each class gets 6 students)
  console.log('\n[6/12] 分配学生到班级 (180 members)...')
  const classNameList = Object.keys(classMap)
  let membershipCount = 0
  for (let ci = 0; ci < classNameList.length; ci++) {
    const cid = classMap[classNameList[ci]]
    const start = (ci * 6) % studentIds.length
    for (let si = 0; si < 6; si++) {
      const sid = studentIds[(start + si) % studentIds.length]
      try { run('INSERT OR IGNORE INTO class_memberships (class_id, student_id) VALUES (?,?)', cid, sid); membershipCount++ } catch {}
    }
  }
  console.log(`  ✓ 班级成员: ${membershipCount}`)

  // Step 8: Create 14 tasks + dimensions + task_classes
  console.log('\n[7/12] 创建 14 个实训任务...')
  const taskMap = {} // taskName → id
  const taskClassMap = {} // taskName → [classIds]

  for (const t of TASKS) {
    const courseId = courseMap[COURSES[t.course_idx].name]
    const tid = teacherMap[t.teacher_idx]
    const deadline = t.deadline_days !== null
      ? (t.deadline_days > 0 ? daysFromNow(t.deadline_days) : daysAgo(-t.deadline_days))
      : null
    run(`INSERT OR IGNORE INTO training_tasks (name, description, requirements, evaluation_criteria, teacher_id, course_id, status, deadline) VALUES (?,?,?,?,?,?,?,?)`,
      t.name, t.description, t.requirements, t.eval_criteria, tid, courseId, t.status, deadline)

    const row = get('SELECT id FROM training_tasks WHERE name=? AND teacher_id=?', t.name, tid)
    const taskId = row.id
    taskMap[t.name] = taskId

    // Dimensions
    for (let di = 0; di < t.dims.length; di++) {
      run('INSERT OR IGNORE INTO dimensions (task_id, name, weight, order_index) VALUES (?,?,?,?)',
        taskId, t.dims[di].name, t.dims[di].w, di)
    }

    // Assign to classes (2-4 random classes belonging to this teacher)
    const eligibleClasses = classNameList.filter((cn, ci) => ci % 3 === t.teacher_idx)
    const assigned = pickN(eligibleClasses, Math.min(4, eligibleClasses.length))
    taskClassMap[t.name] = assigned.map(cn => classMap[cn])
    for (const cid of taskClassMap[t.name]) {
      run('INSERT OR IGNORE INTO task_classes (task_id, class_id) VALUES (?,?)', taskId, cid)
    }
    console.log(`  [${Object.keys(taskMap).length}/14] ✓ ${t.name}`)
  }

  // Step 9: Create uploads + evaluations + dimension_scores
  console.log('\n[8/12] 创建提交、评价和维度评分...')
  let upCount = 0, evCount = 0, dsCount = 0

  const uploadSQL = `INSERT OR IGNORE INTO uploads (task_id, student_id, filename, file_type, file_size, storage_path, sha256, parse_status, version, created_at) VALUES (?,?,?,?,?,?,?,?,1,?)`
  const evalSQL = `INSERT OR IGNORE INTO evaluations (task_id, student_id, upload_id, status, total_score, overall_comment, objective_ratio, created_at) VALUES (?,?,?,?,?,?,?,?)`
  const dimScoreSQL = `INSERT OR IGNORE INTO dimension_scores (evaluation_id, dimension_id, ai_score, teacher_score, rationale) VALUES (?,?,?,?,?)`

  for (const t of TASKS) {
    if (t.status === 'draft') continue
    const taskId = taskMap[t.name]

    // Get enrolled students via class memberships of linked classes
    const classIds = taskClassMap[t.name] || []
    const enrolledSet = new Set()
    for (const cid of classIds) {
      const members = all('SELECT student_id FROM class_memberships WHERE class_id=?', cid)
      for (const m of members) enrolledSet.add(m.student_id)
    }
    const enrolled = [...enrolledSet]
    if (enrolled.length === 0) continue

    // Submission rate: closed=100%, published=70-90%
    const rate = t.status === 'closed' ? 1.0 : 0.7 + Math.random() * 0.2
    const submitters = pickN(enrolled, Math.ceil(enrolled.length * rate))

    for (let si = 0; si < submitters.length; si++) {
      const studentId = submitters[si]
      // Generate upload
      const uploadDays = t.status === 'closed'
        ? Math.max(Math.abs(t.deadline_days) + rand(-3, 1), 1)
        : rand(1, Math.max(Math.abs(t.deadline_days || 7), 2))
      const uploadTime = daysAgo(uploadDays)
      const ext = ['pdf', 'zip', 'docx'][rand(0, 2)]
      const sanitized = t.name.replace(/[\/\\:*?"<>|']/g, '_')
      const sha = `seed_${taskId}_${studentId}_${si}`

      run(uploadSQL, taskId, studentId, sanitized, ext, rand(80000, 500000), `task_${taskId}/s${studentId}/v1.${ext}`, sha, 'parsed', uploadTime)
      upCount++

      const uploadRow = get('SELECT id FROM uploads WHERE sha256=?', sha)
      if (!uploadRow) continue
      const uploadId = uploadRow.id

      // Generate evaluation
      const evalDays = Math.max(uploadDays - rand(0, 3), 0)
      const evalTime = evalDays > 0 ? daysAgo(evalDays) : uploadTime
      const baseScore = 55 + Math.random() * 40 // 55-95
      const noise = (Math.random() - 0.5) * 16
      const totalScore = clamp(Math.round((baseScore + noise) * 10) / 10, 35, 100)
      const comment = totalScore >= 90 ? '优秀，完成质量高' : totalScore >= 80 ? '良好，整体不错' : totalScore >= 70 ? '中等，有改进空间' : totalScore >= 60 ? '及格，需要加强' : '不及格，需重做'
      const objRatio = clamp(randf(0.4, 0.9, 2), 0, 1)
      const evalStatus = t.status === 'closed' ? 'confirmed' : (si % 3 === 0 ? 'confirmed' : 'scored')

      run(evalSQL, taskId, studentId, uploadId, evalStatus, totalScore, comment, objRatio, evalTime)
      evCount++

      const evalRow = get('SELECT id FROM evaluations WHERE upload_id=?', uploadId)
      if (!evalRow) continue
      const evalId = evalRow.id

      // Dimension scores
      for (let di = 0; di < t.dims.length; di++) {
        const dimName = t.dims[di].name
        const dimRow = get('SELECT id FROM dimensions WHERE task_id=? AND name=? ORDER BY order_index LIMIT 1', taskId, dimName)
        if (!dimRow) continue
        const dimId = dimRow.id

        const dimBase = totalScore + (Math.random() - 0.5) * 20
        const aiScore = clamp(Math.round(dimBase * 10) / 10, 30, 100)
        const tScore = evalStatus === 'confirmed'
          ? clamp(Math.round((dimBase + (Math.random() - 0.5) * 10) * 10) / 10, 30, 100)
          : null
        const rationale = getCritique(dimName)

        run(dimScoreSQL, evalId, dimId, aiScore, tScore, rationale)
        dsCount++
      }
    }
    console.log(`  ✓ ${t.name}: ${submitters.length} 份提交`)
  }
  console.log(`  ├ 提交: ${upCount} | 评价: ${evCount} | 维度评分: ${dsCount}`)

  // Step 10: Similarity records
  console.log('\n[9/12] 创建相似度记录...')
  let simCount = 0
  const simSQL = 'INSERT OR IGNORE INTO similarity_records (task_id, upload_a_id, upload_b_id, hamming_distance, cosine_similarity, state) VALUES (?,?,?,?,?,?)'
  for (const t of TASKS) {
    if (t.status === 'draft') continue
    const taskId = taskMap[t.name]
    const uploads = all('SELECT id FROM uploads WHERE task_id=? ORDER BY random() LIMIT 8', taskId)
    if (uploads.length < 2) continue
    const pairs = Math.min(3, Math.floor(uploads.length / 2))
    for (let i = 0; i < pairs; i++) {
      const a = uploads[i * 2].id
      const b = uploads[i * 2 + 1].id
      const aId = Math.min(a, b)
      const bId = Math.max(a, b)
      const ham = rand(3, 25)
      const cos = randf(0.5, 0.98, 2)
      const state = cos > 0.85 ? 'suspect' : (cos > 0.65 ? 'suspect' : 'ignored')
      try { run(simSQL, taskId, aId, bId, ham, cos, state); simCount++ } catch {}
    }
  }
  console.log(`  ✓ 相似度记录: ${simCount}`)

  // Step 11: Notifications (丰厚的通知数据)
  console.log('\n[10/12] 创建通知...')
  let notifCount = 0
  const notifSQL = 'INSERT INTO notifications (user_id, type, title, content, is_read, created_at) VALUES (?,?,?,?,?,?)'

  // Teacher notifications - 4 per teacher
  const teacherNotifTemplates = [
    { type: 'EVALUATION_COMPLETED', title: '批量AI评分已完成', content: '系统已完成多份提交的AI自动评分，请前往批改工作台确认。' },
    { type: 'SIMILARITY_DETECTED', title: '⚠️ 高相似度预警', content: '检测到有提交之间存在异常高相似度（>85%），建议立即人工复核。' },
    { type: 'DEADLINE_APPROACHING', title: '⏰ 任务即将截止', content: '距离截止日期还有3天，尚有部分学生未提交实训报告。' },
    { type: 'EVALUATION_COMPLETED', title: '📊 班级评分统计', content: '已确认评价的班级平均分分布已生成，可查看详细统计分析。' },
  ]
  for (const tid of Object.values(teacherMap)) {
    for (const nt of teacherNotifTemplates) {
      run(notifSQL, tid, nt.type, nt.title, nt.content, rand(0, 1), daysAgo(randf(0, 7, 2)))
      notifCount++
    }
  }

  // Admin notifications - 6
  const adminNotifs = [
    { type: 'SYSTEM_STATS', title: '📈 系统运行周报', content: '本周新增评价数据量正常，系统运行稳定，无异常告警。' },
    { type: 'EVALUATION_COMPLETED', title: '评价统计', content: `系统累计完成 ${evCount} 份自动评价。` },
    { type: 'USER_ACTIVITY', title: '👥 用户活跃度报告', content: '本周活跃教师3人，活跃学生156人，整体活跃度良好。' },
    { type: 'TASK_PUBLISHED', title: '📋 任务发布统计', content: '本周教师共发布6个新实训任务，覆盖4门课程。' },
    { type: 'LLM_STATUS', title: '🤖 LLM服务状态', content: 'AI评阅服务运行正常，平均响应时间正常。' },
    { type: 'SYSTEM_STATS', title: '🛡️ 安全报告', content: '本周系统未检测到异常登录行为，安全状态良好。' },
  ]
  for (const nt of adminNotifs) {
    run(notifSQL, adminId, nt.type, nt.title, nt.content, rand(0, 1), daysAgo(randf(0, 7, 2)))
    notifCount++
  }

  // Student notifications - first 60 students get 2 each
  let stuNotifCount = 0
  for (let i = 0; i < Math.min(60, studentIds.length); i++) {
    const sid = studentIds[i]
    const notifs = [
      { type: 'EVALUATION_COMPLETED', title: '📝 新评价已生成', content: '你的实训报告评价已出，点击查看详细评分和评语。' },
      { type: 'TASK_REMINDER', title: '📌 新任务提醒', content: '你有新的实训任务待完成，请及时查看并提交。' },
    ]
    for (const nt of notifs) {
      try { run(notifSQL, sid, nt.type, nt.title, nt.content, rand(0, 1), daysAgo(randf(0, 7, 2))); stuNotifCount++ } catch {}
    }
  }
  notifCount += stuNotifCount
  console.log(`  ✓ 通知: ${notifCount}`)

  // Step 12: Audit logs (500+ diverse logs)
  console.log('\n[11/12] 创建审计日志...')
  const allUsers = all("SELECT id, username, role FROM users WHERE is_active=1 ORDER BY id")
  const admins = allUsers.filter(u => u.role === 'admin')
  const teachersList = allUsers.filter(u => u.role === 'teacher')
  const studentsList = allUsers.filter(u => u.role === 'student')

  const auditDefs = [
    { action: 'auth.login', detail: '登录系统', pool: 'all' },
    { action: 'auth.login', detail: '登录成功', pool: 'all' },
    { action: 'upload.created', detail: '提交实验报告', pool: 'student' },
    { action: 'upload.created', detail: '上传实训作业', pool: 'student' },
    { action: 'evaluation.auto_scored', detail: 'AI自动评分完成', pool: 'teacher' },
    { action: 'evaluation.confirmed', detail: '确认评价结果', pool: 'teacher' },
    { action: 'evaluation.rejected', detail: '退回评价要求重审', pool: 'teacher' },
    { action: 'task.published', detail: '发布新实训任务', pool: 'teacher' },
    { action: 'task.closed', detail: '关闭实训任务', pool: 'teacher' },
    { action: 'auth.login.failed', detail: '密码错误', pool: 'all' },
    { action: 'auth.login.failed', detail: '登录失败（账号锁定）', pool: 'all' },
    { action: 'user.created', detail: '批量导入学生用户', pool: 'admin' },
    { action: 'user.created', detail: '创建教师账号', pool: 'admin' },
    { action: 'llm.config.updated', detail: '更新LLM模型配置', pool: 'admin' },
    { action: 'llm.config.updated', detail: '切换AI评分模型', pool: 'admin' },
    { action: 'chat.message', detail: 'AI助教对话', pool: 'student' },
    { action: 'chat.message', detail: '查询评价数据', pool: 'student' },
    { action: 'similarity.check', detail: '查重检测完成', pool: 'teacher' },
    { action: 'similarity.decided', detail: '确认查重结果', pool: 'teacher' },
    { action: 'report.generated', detail: '生成学生画像报告', pool: 'teacher' },
    { action: 'report.generated', detail: '导出班级成绩统计', pool: 'teacher' },
    { action: 'dashboard.viewed', detail: '查看仪表盘', pool: 'all' },
    { action: 'profile.updated', detail: '更新个人信息', pool: 'all' },
  ]
  const ips = ['192.168.1.100','192.168.1.101','192.168.1.102','192.168.1.103','192.168.1.104','192.168.1.105','10.0.0.55','10.0.1.20','172.16.0.10','172.16.0.11','192.168.2.50','192.168.2.51']

  const auditSQL = 'INSERT INTO audit_logs (occurred_at, user_id, username, role, action, target_type, target_id, result, detail, client_ip) VALUES (?,?,?,?,?,?,?,?,?,?)'
  let auditCount = 0
  for (let i = 0; i < 500; i++) {
    const def = pick(auditDefs)
    let pool
    if (def.pool === 'admin') pool = admins
    else if (def.pool === 'teacher') pool = teachersList
    else if (def.pool === 'student') pool = studentsList
    else pool = allUsers
    if (pool.length === 0) continue
    const user = pick(pool)
    const ts = daysAgo(randf(0, 14, 2))
    const targetType = def.action.split('.')[0]
    const targetId = rand(1, 300)
    const result = def.action.includes('failed') ? 'failed' : (Math.random() > 0.05 ? 'success' : 'failed')

    try {
      run(auditSQL, ts, user.id, user.username, user.role, def.action, targetType, targetId, result, def.detail, pick(ips))
      auditCount++
    } catch {}
  }
  console.log(`  ✓ 审计日志: ${auditCount}`)

  // Step 13: Student profiles (for up to 80 students)
  console.log('\n[12/12] 创建学生画像...')
  let profileCount = 0
  const profileSQL = `INSERT OR REPLACE INTO student_profiles (student_id, radar_data, weakness_list, suggestions, score_trend, source_evaluation_count, computed_at) VALUES (?,?,?,?,?,?,?)`

  for (let i = 0; i < Math.min(80, studentIds.length); i++) {
    const sid = studentIds[i]
    // Try to get real dimension score data
    const dimRows = all(`SELECT d.name, AVG(ds.teacher_score) as avg_score
      FROM dimension_scores ds JOIN dimensions d ON ds.dimension_id = d.id
      JOIN evaluations e ON ds.evaluation_id = e.id
      WHERE e.student_id=? AND ds.teacher_score IS NOT NULL
      GROUP BY d.name LIMIT 6`, sid)
    let radarObj
    if (dimRows.length >= 2) {
      const entries = dimRows.map(r => `"${r.name}": ${Math.round(r.avg_score)}`)
      radarObj = '{' + entries.join(',') + '}'
    } else {
      const dims = [
        '"专业知识":', '"实践能力":', '"创新能力":', '"文档能力":', '"团队协作":', '"表达能力":'
      ]
      radarObj = '{' + dims.map(d => `${d}${rand(60, 95)}`).join(',') + '}'
    }

    const evalCount = get('SELECT COUNT(*) as c FROM evaluations WHERE student_id=?', sid).c
    const avgScore = get('SELECT COALESCE(AVG(total_score),0) as avg FROM evaluations WHERE student_id=?', sid).avg

    // Generate weaknesses (2 items below 75 or lowest scores)
    const weaknessItems = []
    if (avgScore < 75) weaknessItems.push({ name: '综合成绩偏低', score: Math.round(avgScore) })
    else weaknessItems.push({ name: '创新能力', score: rand(60, 78) })
    weaknessItems.push({ name: '文档规范性', score: rand(63, 80) })

    // Generate suggestions
    const suggestionPool = [
      '建议加强代码实践，多参与开源项目',
      '注重实验报告的结构化和规范化',
      '加强理论知识学习，结合实际应用',
      '建议多和同学交流讨论，开拓思路',
      '可以在创新性方面多下功夫',
      '建议定期复习已学知识，形成体系',
      '加强时间管理，提前规划实验进度',
      '建议参加学科竞赛提升实践能力',
    ]
    const suggestions = JSON.stringify(pickN(suggestionPool, 3))

    // Generate trend
    const t1 = clamp(avgScore - rand(8, 18), 40, 95)
    const t2 = clamp(avgScore - rand(3, 10), 45, 97)
    const t3 = clamp(avgScore + rand(0, 5), 50, 100)
    const trend = JSON.stringify([
      { period: '实训1', score: Math.round(t1) },
      { period: '实训2', score: Math.round(t2) },
      { period: '实训3', score: Math.round(t3) },
    ])

    try {
      run(profileSQL, sid, radarObj, JSON.stringify(weaknessItems), suggestions, trend, Math.max(evalCount, 1), daysAgo(randf(0, 2, 2)))
      profileCount++
    } catch {}
  }
  console.log(`  ✓ 学生画像: ${profileCount}`)

  // ─── Final Report ───
  console.log('\n═══════════════════════════════════════════')
  console.log('📊  最终数据统计')
  console.log('═══════════════════════════════════════════\n')
  const tables = [
    ['用户总数', 'users', ''],
    ['  管理员', 'users', "role='admin'"],
    ['  教师', 'users', "role='teacher'"],
    ['  学生', 'users', "role='student'"],
    ['课程', 'courses', ''],
    ['班级', 'classes', ''],
    ['班级成员', 'class_memberships', ''],
    ['实训任务', 'training_tasks', ''],
    ['  已发布', 'training_tasks', "status='published'"],
    ['  草稿', 'training_tasks', "status='draft'"],
    ['  已关闭', 'training_tasks', "status='closed'"],
    ['评价维度', 'dimensions', ''],
    ['提交记录', 'uploads', ''],
    ['评价记录', 'evaluations', ''],
    ['维度评分', 'dimension_scores', ''],
    ['相似度记录', 'similarity_records', ''],
    ['通知', 'notifications', ''],
    ['审计日志', 'audit_logs', ''],
    ['学生画像', 'student_profiles', ''],
  ]
  for (const [label, table, where] of tables) {
    const sql = where ? `SELECT COUNT(*) as c FROM ${table} WHERE ${where}` : `SELECT COUNT(*) as c FROM ${table}`
    const cnt = get(sql).c
    console.log(`  ${label.padEnd(12)} ${String(cnt).padStart(5)}`)
  }

  console.log('\n✅ 种子数据填充完成！')
  console.log('')
  console.log('📋 教师账号:')
  console.log(`  ${TEACHERS.map(t => `${t.username} / test123`).join('\n  ')}`)
  console.log(`\n📋 学生: student2 ~ student181 / test123`)
  console.log(`  (共 ${stuCount} 名学生)`)
  console.log(`\n📋 管理员: admin / admin123`)

  db.close()
  console.timeEnd('seed-total')
}

main()