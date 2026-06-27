#!/usr/bin/env node
/**
 * 数据完整性全面验证脚本
 * 检查所有实体、外键、业务逻辑一致性
 */
import { DatabaseSync } from 'node:sqlite'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const DB_PATH = path.resolve(__dirname, '../go-backend/data/app.db')
const db = new DatabaseSync(DB_PATH)

const errors = []
const warnings = []

function check(desc, condition, detail = '') {
  if (condition) {
    console.log('  ✅ ' + desc)
  } else {
    console.log('  ❌ ' + desc + (detail ? ' — ' + detail : ''))
    errors.push(desc + (detail ? ': ' + detail : ''))
  }
}
function warn(desc, detail = '') {
  console.log('  ⚠️  ' + desc + (detail ? ' — ' + detail : ''))
  warnings.push(desc + (detail ? ': ' + detail : ''))
}
function q(sql) { return db.prepare(sql).all() }
function q1(sql) { const r = q(sql); return r.length > 0 ? r[0] : null }

console.log('╔═══════════════════════════════════════════════╗')
console.log('║    数据完整性全面验证                         ║')
console.log('╚═══════════════════════════════════════════════╝\n')

// ═══════ 1. 用户数据 ═══════
console.log('【1. 用户数据】')
const users = q('SELECT * FROM users ORDER BY id')
const admins = users.filter(u => u.role === 'admin')
const teachers = users.filter(u => u.role === 'teacher')
const students = users.filter(u => u.role === 'student')

check('用户总数 >= 220', users.length >= 220, `实际: ${users.length}`)
check('管理员 >= 1', admins.length >= 1, `实际: ${admins.length}`)
check('教师 >= 4', teachers.length >= 4, `实际: ${teachers.length}`)
check('学生 >= 200', students.length >= 200, `实际: ${students.length}`)

const noHash = users.filter(u => !u.password_hash)
check('所有用户有密码哈希', noHash.length === 0, `${noHash.length} 个缺密码`)

const badRole = users.filter(u => !['admin','teacher','student'].includes(u.role))
check('所有角色合法', badRole.length === 0, `${badRole.length} 个角色异常`)

// ═══════ 2. 课程 ═══════
console.log('\n【2. 课程数据】')
const courses = q('SELECT * FROM courses')
check('课程 >= 8', courses.length >= 8, `实际: ${courses.length}`)
check('所有课程有 code', courses.filter(c => !c.code).length === 0)
check('所有课程有 name', courses.filter(c => !c.name).length === 0)

// ═══════ 3. 班级 ═══════
console.log('\n【3. 班级数据】')
const classes = q('SELECT * FROM classes')
check('班级 >= 30', classes.length >= 30, `实际: ${classes.length}`)
check('所有班级有 teacher_id', classes.filter(c => !c.teacher_id).length === 0)
check('所有班级有 course_id', classes.filter(c => !c.course_id).length === 0)

// 教师外键
for (const cl of classes) {
  const t = users.find(u => u.id === cl.teacher_id && u.role === 'teacher')
  if (!t) { check(`班级 ${cl.id} teacher_id=${cl.teacher_id} 非有效教师`, false); break }
}

// 班级成员
const cm = q(`
  SELECT cm.class_id, c.name, COUNT(*) as cnt
  FROM class_memberships cm
  JOIN classes c ON cm.class_id = c.id
  GROUP BY cm.class_id
`)
const totalMembers = cm.reduce((s, r) => s + r.cnt, 0)
const zeroMemberCls = cm.filter(r => r.cnt === 0)
check('所有班级有成员', zeroMemberCls.length === 0)
check('班级成员总数 >= 180', totalMembers >= 180, `实际: ${totalMembers}`)
const minMem = Math.min(...cm.map(r => r.cnt))
const maxMem = Math.max(...cm.map(r => r.cnt))
check('每班至少 1 人', minMem >= 1, `最少: ${minMem}`)
if (maxMem < 3) warn('最大班级人数偏少')

// ═══════ 4. 任务 ═══════
console.log('\n【4. 实训任务数据】')
const tasks = q('SELECT * FROM training_tasks ORDER BY id')
check('任务 >= 14', tasks.length >= 14, `实际: ${tasks.length}`)

const pubT = tasks.filter(t => t.status === 'published').length
const cloT = tasks.filter(t => t.status === 'closed').length
const draT = tasks.filter(t => t.status === 'draft').length
check('有已发布任务', pubT >= 6, pubT)
check('有关闭任务', cloT >= 2, cloT)
check('有草稿任务', draT >= 1, draT)
check('状态合法', pubT + cloT + draT === tasks.length)

check('所有任务有 teacher_id', tasks.filter(t => !t.teacher_id).length === 0)
check('所有任务有 course_id', tasks.filter(t => !t.course_id).length === 0)

const noDeadline = tasks.filter(t => t.status !== 'draft' && !t.deadline)
check('已发布/关闭任务有 deadline', noDeadline.length === 0, `${noDeadline.length} 个缺截止日期`)

for (const t of teachers) {
  const cnt = tasks.filter(tk => tk.teacher_id === t.id).length
  const row = q1(`SELECT display_name FROM users WHERE id = ${t.id}`)
  if (cnt < 2) check(`${row.display_name} 任务 >= 2`, false, `仅 ${cnt} 个`)
  else check(`${row.display_name} 有 ${cnt} 个任务`, true)
}

// ═══════ 5. 维度 ═══════
console.log('\n【5. 评价维度数据】')
const dims = q('SELECT d.* FROM dimensions d')
check('维度 >= 50', dims.length >= 50, `实际: ${dims.length}`)
const badW = dims.filter(d => !d.weight || d.weight < 1 || d.weight > 100)
check('维度权重 1-100', badW.length === 0, `${badW.length} 个异常`)

// 每任务权重和 ≈ 100
const taskDimSum = {}
for (const d of dims) {
  if (!taskDimSum[d.task_id]) taskDimSum[d.task_id] = 0
  taskDimSum[d.task_id] += d.weight
}
for (const [tid, sum] of Object.entries(taskDimSum)) {
  if (sum < 80 || sum > 100) warn(`任务 ${tid} 权重和 = ${sum}`)
}

// ═══════ 6. 提交 ═══════
console.log('\n【6. 提交数据】')
const uploads = q('SELECT u.* FROM uploads u')
check('提交 >= 200', uploads.length >= 200, `实际: ${uploads.length}`)
check('所有提交有 student_id', uploads.filter(u => !u.student_id).length === 0)
check('所有提交有 filename', uploads.filter(u => !u.filename).length === 0)

const notParsed = uploads.filter(u => u.parse_status !== 'parsed')
check('所有提交状态 parsed', notParsed.length === 0, `${notParsed.length} 个未解析`)

// 外键驻留
let orphanUp = 0
for (const u of uploads) {
  const s = students.find(st => st.id === u.student_id)
  if (!s) orphanUp++
}
check('提交无孤立外键', orphanUp === 0, `${orphanUp} 个孤立`)

// 无重复(task_id + student_id)
const upPairs = new Set()
let dupUp = 0
for (const u of uploads) {
  const key = `${u.task_id}-${u.student_id}`
  if (upPairs.has(key)) dupUp++
  upPairs.add(key)
}
check('提交无重复(同任务+同学生)', dupUp === 0, `${dupUp} 个重复`)

// ═══════ 7. 评价 ═══════
console.log('\n【7. 评价数据】')
const evals = q('SELECT e.* FROM evaluations e')
check('评价 >= 200', evals.length >= 200, `实际: ${evals.length}`)

const confE = evals.filter(e => e.status === 'confirmed').length
const scorE = evals.filter(e => e.status === 'scored').length
const pendE = evals.filter(e => e.status === 'pending').length
check('有已确认评价', confE >= 30, confE)
check('有已评分评价', scorE >= 30, scorE)
check('评价状态合法', confE + scorE + pendE === evals.length)

const nullScore = evals.filter(e => e.total_score === null)
if (nullScore.length > 0) warn(`${nullScore.length} 个评价总分为 null**`)

const outRange = evals.filter(e => e.total_score !== null && (e.total_score < 0 || e.total_score > 100))
check('总分在 0-100', outRange.length === 0, `${outRange.length} 个超出`)

const uniqScores = new Set(evals.filter(e => e.total_score !== null).map(e => Math.round(e.total_score)))
check('分数分布多样化', uniqScores.size >= 15, `不同分值: ${uniqScores.size}`)

const noUpEval = evals.filter(e => !e.upload_id)
check('所有评价有 upload_id', noUpEval.length === 0)
check('所有评价有 upload_id', noUpEval.length === 0)

const noRatio = evals.filter(e => e.objective_ratio === null)
check('所有评价有 objective_ratio', noRatio.length === 0, `${noRatio.length} 个 null`)

// ═══════ 8. 维度评分 ═══════
console.log('\n【8. 维度评分数据】')
const dscores = q('SELECT ds.* FROM dimension_scores ds')
check('维度评分 >= 400', dscores.length >= 400, `实际: ${dscores.length}`)

const badAi = dscores.filter(d => d.ai_score === null || d.ai_score < 0 || d.ai_score > 100)
check('ai_score 在 0-100', badAi.length === 0, `${badAi.length} 个异常`)

const badTs = dscores.filter(d => d.teacher_score !== null && (d.teacher_score < 0 || d.teacher_score > 100))
check('teacher_score 在 0-100', badTs.length === 0, `${badTs.length} 个异常`)

const noRat = dscores.filter(d => !d.rationale)
check('有评语', noRat.length === 0, `${noRat.length} 个无评语`)

const uniqRat = new Set(dscores.filter(d => d.rationale).map(d => d.rationale))
check('评语多样化', uniqRat.size >= 10, `不同评语: ${uniqRat.size}`)

// 维度评分数量匹配
let dimMismatch = 0
for (const ev of evals) {
  const taskDims = dims.filter(d => d.task_id === ev.task_id)
  const evalDims = dscores.filter(d => d.evaluation_id === ev.id)
  if (taskDims.length > 0 && evalDims.length !== taskDims.length) {
    dimMismatch++
    if (dimMismatch <= 3) warn(`评价 ${ev.id} 维度评分数(${evalDims.length}) != 任务维度(${taskDims.length})`)
  }
}
check('维度评分与任务维度匹配', dimMismatch === 0, `${dimMismatch} 个不匹配`)

// ═══════ 9. 相似度 ═══════
console.log('\n【9. 相似度数据】')
const sims = q('SELECT * FROM similarity_records')
check('相似度 >= 20', sims.length >= 20, `实际: ${sims.length}`)

const simState = sims.filter(s => ['suspect','confirmed','ignored'].includes(s.state)).length
check('状态合法', simState === sims.length, `${sims.length - simState} 个状态异常`)

check('有 suspect 记录', sims.filter(s => s.state === 'suspect').length >= 3)
check('hamming_distance 有效', sims.filter(s => s.hamming_distance === null || s.hamming_distance < 0).length === 0)
const badCos = sims.filter(s => s.cosine_similarity === null || s.cosine_similarity < 0 || s.cosine_similarity > 1)
check('cosine_similarity 在 0-1', badCos.length === 0, `${badCos.length} 个异常`)

// ═══════ 10. 审计日志 ═══════
console.log('\n【10. 审计日志数据】')
const audits = q('SELECT * FROM audit_logs')
check('审计 >= 400', audits.length >= 400, `实际: ${audits.length}`)

const uniqActs = new Set(audits.map(a => a.action))
check('动作多样化(>= 8种)', uniqActs.size >= 8, `实际: ${uniqActs.size}`)

const auditDates = audits.filter(a => a.occurred_at).map(a => a.occurred_at.substring(0, 10))
const uniqDates = new Set(auditDates)
check('跨多天(>= 3)', uniqDates.size >= 3, `实际: ${uniqDates.size} 天`)

// ═══════ 11. 通知 ═══════
console.log('\n【11. 通知数据】')
const notifs = q('SELECT * FROM notifications')
check('通知 >= 50', notifs.length >= 50, `实际: ${notifs.length}`)

const adminN = notifs.filter(n => n.user_id === 1).length
const teacherIds = new Set(teachers.map(t => t.id))
const studentIdsSet = new Set(students.map(s => s.id))
const teacherN = notifs.filter(n => teacherIds.has(n.user_id)).length
const studentN = notifs.filter(n => studentIdsSet.has(n.user_id)).length
check('管理员通知 >= 3', adminN >= 3, adminN)
check('教师通知 >= 5', teacherN >= 5, teacherN)
check('学生通知 >= 10', studentN >= 10, studentN)

const notifTypes = new Set(notifs.map(n => n.type))
check('通知类型多样化(>= 3)', notifTypes.size >= 3, `实际: ${notifTypes.size}`)

// ═══════ 12. 学生画像 ═══════
console.log('\n【12. 学生画像数据】')
const profiles = q('SELECT * FROM student_profiles')
check('学生画像 >= 30', profiles.length >= 30, `实际: ${profiles.length}`)

let emptyRadar = 0, emptyWeak = 0, emptySugg = 0, emptyTrend = 0
for (const p of profiles) {
  if (!p.radar_data || p.radar_data === '{}' || p.radar_data === '[]') emptyRadar++
  if (!p.weakness_list || p.weakness_list === '[]') emptyWeak++
  if (!p.suggestions || p.suggestions === '[]') emptySugg++
  if (!p.score_trend || p.score_trend === '[]') emptyTrend++
}
if (emptyRadar > 0) warn(`${emptyRadar} 个画像 radar_data 为空`)
if (emptyWeak > 0) warn(`${emptyWeak} 个画像 weakness_list 为空`)
if (emptySugg > 0) warn(`${emptySugg} 个画像 suggestions 为空`)
if (emptyTrend > 0) warn(`${emptyTrend} 个画像 score_trend 为空`)

// ═══════ 13. 业务逻辑一致性 ═══════
console.log('\n【13. 业务逻辑一致性】')

// 已关闭任务无 pending 评价
for (const t of tasks.filter(tk => tk.status === 'closed')) {
  const p = evals.filter(e => e.task_id === t.id && e.status === 'pending')
  if (p.length > 0) warn(`已关闭任务 ${t.id} 有 ${p.length} 个 pending 评价`)
}

// 草稿任务无提交
for (const t of tasks.filter(tk => tk.status === 'draft')) {
  const up = uploads.filter(u => u.task_id === t.id)
  if (up.length > 0) warn(`草稿任务 ${t.name} 有 ${up.length} 个提交`)
}

// 每个任务 task_classes 关联存在
const tclass = q('SELECT COUNT(*) as c FROM task_classes')
check('有关联表 task_classes', tclass[0].c > 0, tclass[0].c)

// 总统计
console.log('\n═══════════════════════════════════════════════')
console.log('📊  验证报告')
console.log('═══════════════════════════════════════════════\n')
console.log(`  总用户:     ${users.length}`)
console.log(`   管理员:    ${admins.length}`)
console.log(`   教师:      ${teachers.length}`)
console.log(`   学生:      ${students.length}`)
console.log(`  课程:       ${courses.length}`)
console.log(`  班级:       ${classes.length}`)
console.log(`  班级成员:   ${totalMembers}`)
console.log(`  任务:       ${tasks.length} (发布${pubT} 草稿${draT} 关闭${cloT})`)
console.log(`  维度:       ${dims.length}`)
console.log(`  提交:       ${uploads.length}`)
console.log(`  评价:       ${evals.length} (确认${confE} 评分${scorE} 待批${pendE})`)
console.log(`  维度评分:   ${dscores.length}`)
console.log(`  相似度:     ${sims.length}`)
console.log(`  通知:       ${notifs.length}`)
console.log(`  审计日志:   ${audits.length}`)
console.log(`  学生画像:   ${profiles.length}`)
console.log('')
console.log(`  错误:       ${errors.length}`)
console.log(`  警告:       ${warnings.length}`)
console.log('')

if (errors.length === 0) {
  console.log('🎉 所有关键检查通过！数据完整且一致。')
} else {
  console.log(`❌ 发现 ${errors.length} 个错误:`)
  for (const e of errors) console.log(`   - ${e}`)
}
if (warnings.length > 0) {
  console.log(`\n⚠️  ${warnings.length} 个警告（仅提示）:`)
  for (const w of warnings) console.log(`   - ${w}`)
}

db.close()