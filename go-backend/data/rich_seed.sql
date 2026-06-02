-- ============================================================
-- Rich seed: 纯净模拟数据，让所有角色仪表盘丰满好看
-- 执行前提：先 go run ./cmd/seed 建立 admin/teacher1/student1 + 课程 SE101
-- 密码统一 test123（bcrypt cost=10）
-- ============================================================

-- ============================================================
-- 额外教师
-- ============================================================
INSERT OR IGNORE INTO users (username, display_name, password_hash, role, is_active) VALUES
('teacher2', '李教授', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'teacher', 1),
('teacher3', '陈副教授', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'teacher', 1);

-- ============================================================
-- 25 名学生
-- ============================================================
INSERT OR IGNORE INTO users (username, display_name, password_hash, role, is_active) VALUES
('student2', '王小明', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student3', '赵文博', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student4', '刘思琪', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student5', '孙浩然', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student6', '周雨萱', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student7', '吴子轩', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student8', '郑雅琪', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student9', '黄俊杰', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student10', '林诗涵', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student11', '何宇航', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student12', '马晓彤', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student13', '罗天宇', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student14', '梁欣怡', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student15', '宋嘉豪', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student16', '谢雨桐', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student17', '韩泽宇', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student18', '唐悦彤', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student19', '冯子涵', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1),
('student20', '曹明轩', '$2a$10$2nwBA.EADPJHxvxmJcDxoOkrLDK.wRPJ0t42oZ.dOwnYeo7dw2xo2', 'student', 1);

-- ============================================================
-- 课程（共4门）
-- ============================================================
INSERT OR IGNORE INTO courses (name, code) VALUES
('计算机网络', 'CN201'),
('数据结构与算法', 'DS301'),
('操作系统原理', 'OS401');

-- ============================================================
-- 班级（6个，绑定到教师）
-- ============================================================
INSERT OR IGNORE INTO classes (name, course_id, teacher_id, student_count) VALUES
('软工21-1班', 1, 2, 15),
('软工21-2班', 1, 2, 12),
('网络22-1班', 2, 4, 14),
('数据结构23-1班', 3, 4, 13),
('操作系统22-1班', 4, 5, 10),
('软工21-3班', 1, 2, 8);

-- ============================================================
-- 班级成员（student1=id3 开始）
-- ============================================================
INSERT OR IGNORE INTO class_memberships (class_id, student_id) VALUES
(1, 3),(1, 4),(1, 5),(1, 6),(1, 7),(1, 8),(1, 9),(1, 10),(1, 11),(1, 12),(1, 13),(1, 14),(1, 15),(1, 16),(1, 17),
(2, 3),(2, 6),(2, 7),(2, 8),(2, 9),(2, 10),(2, 11),(2, 12),(2, 13),(2, 14),(2, 15),(2, 16),
(6, 3),(6, 4),(6, 5),(6, 6),(6, 7),(6, 8),(6, 9),(6, 10);

-- ============================================================
-- 实训任务（8个，各种状态）
-- ============================================================
INSERT OR IGNORE INTO training_tasks (name, description, requirements, teacher_id, course_id, status, deadline) VALUES
('并发编程实训', '使用 Java 实现生产者-消费者模型，理解线程同步机制', '1. 源代码 zip\n2. 实验报告 PDF\n3. 测试用例截图', 2, 1, 'published', datetime('now', '+3 days')),
('TCP 协议分析', '使用 Wireshark 抓包分析 TCP 三次握手与四次挥手过程', '1. 抓包文件 .pcap\n2. 分析报告 PDF', 2, 2, 'published', datetime('now', '+7 days')),
('二叉树遍历实现', '实现前序、中序、后序、层序四种遍历算法', '1. Java/Python 源码\n2. 时间复杂度分析', 4, 3, 'published', datetime('now', '+5 days')),
('进程调度模拟', '模拟 FCFS、SJF、RR 三种调度算法并对比性能', '1. 模拟程序源码\n2. 甘特图\n3. 性能对比报告', 5, 4, 'published', datetime('now', '+10 days')),
('RESTful API 设计', '设计一个图书管理系统的 RESTful API 并用 Swagger 文档化', '1. OpenAPI 3.0 YAML\n2. 设计说明文档', 2, 1, 'closed', datetime('now', '-5 days')),
('排序算法对比', '实现快排、归并、堆排序并进行性能基准测试', '1. 源代码\n2. 基准测试结果\n3. 分析报告', 2, 1, 'closed', datetime('now', '-10 days')),
('内存管理实验', '模拟页面置换算法（FIFO、LRU、OPT）', '1. 模拟程序\n2. 缺页率对比图表', 5, 4, 'draft', NULL),
('微服务架构设计', '设计一个电商系统的微服务拆分方案', '1. 架构图\n2. 服务划分文档\n3. API 网关设计', 2, 1, 'draft', NULL);

-- 任务-班级关联
INSERT OR IGNORE INTO task_classes (task_id, class_id) VALUES
(1, 1),(1, 2),(1, 6),(2, 1),(2, 2),(3, 4),(4, 5),(5, 1),(5, 2),(6, 1),(6, 2),(6, 6),(7, 5),(8, 1);

-- ============================================================
-- 评价维度
-- ============================================================
INSERT OR IGNORE INTO dimensions (task_id, name, weight, order_index) VALUES
(1, '代码规范性', 25, 0),(1, '功能完整性', 35, 1),(1, '并发正确性', 25, 2),(1, '文档质量', 15, 3),
(2, '抓包完整性', 30, 0),(2, '分析深度', 40, 1),(2, '报告规范', 30, 2),
(3, '算法正确性', 40, 0),(3, '代码风格', 20, 1),(3, '复杂度分析', 25, 2),(3, '测试覆盖', 15, 3),
(4, '算法实现', 35, 0),(4, '性能对比', 30, 1),(4, '可视化展示', 20, 2),(4, '报告质量', 15, 3),
(5, 'API 设计合理性', 40, 0),(5, '文档完整性', 35, 1),(5, '规范遵循', 25, 2),
(6, '实现正确性', 35, 0),(6, '性能测试', 30, 1),(6, '代码质量', 20, 2),(6, '分析报告', 15, 3);

-- ============================================================
-- 提交数据（分散在近 7 天，让活跃度图表好看）
-- ============================================================
-- Task 1 (并发编程) - 12 submissions spread over 7 days
INSERT OR IGNORE INTO uploads (task_id, student_id, filename, file_type, file_size, storage_path, sha256, parse_status, version, created_at) VALUES
(1, 3, '并发编程_李同学.zip', 'zip', 245000, 'task_1/s3/v1.zip', 'a1b2c3d4', 'parsed', 1, datetime('now', '-6 days')),
(1, 4, '生产者消费者_王小明.pdf', 'pdf', 180000, 'task_1/s4/v1.pdf', 'b2c3d4e5', 'parsed', 1, datetime('now', '-6 days')),
(1, 5, '并发实训_赵文博.zip', 'zip', 320000, 'task_1/s5/v1.zip', 'c3d4e5f6', 'parsed', 1, datetime('now', '-5 days')),
(1, 6, '线程同步_刘思琪.zip', 'zip', 198000, 'task_1/s6/v1.zip', 'd4e5f6g7', 'parsed', 1, datetime('now', '-5 days')),
(1, 7, '并发模型_孙浩然.pdf', 'pdf', 156000, 'task_1/s7/v1.pdf', 'e5f6g7h8', 'parsed', 1, datetime('now', '-4 days')),
(1, 8, '实训三_周雨萱.zip', 'zip', 278000, 'task_1/s8/v1.zip', 'f6g7h8i9', 'parsed', 1, datetime('now', '-4 days')),
(1, 9, '并发编程报告_吴子轩.pdf', 'pdf', 210000, 'task_1/s9/v1.pdf', 'g7h8i9j0', 'parsed', 1, datetime('now', '-3 days')),
(1, 10, '生产者消费者_郑雅琪.zip', 'zip', 345000, 'task_1/s10/v1.zip', 'h8i9j0k1', 'parsed', 1, datetime('now', '-3 days')),
(1, 11, '并发_黄俊杰.zip', 'zip', 189000, 'task_1/s11/v1.zip', 'i9j0k1l2', 'parsed', 1, datetime('now', '-2 days')),
(1, 12, '线程实验_林诗涵.pdf', 'pdf', 167000, 'task_1/s12/v1.pdf', 'j0k1l2m3', 'parsed', 1, datetime('now', '-2 days')),
(1, 13, '并发编程_何宇航.zip', 'zip', 234000, 'task_1/s13/v1.zip', 'k1l2m3n4', 'parsed', 1, datetime('now', '-1 day')),
(1, 14, '线程同步_马晓彤.pdf', 'pdf', 198000, 'task_1/s14/v1.pdf', 'l2m3n4o5', 'parsed', 1, datetime('now', '-1 day'));

-- Task 2 (TCP 协议) - 8 submissions
INSERT OR IGNORE INTO uploads (task_id, student_id, filename, file_type, file_size, storage_path, sha256, parse_status, version, created_at) VALUES
(2, 3, 'TCP分析_李同学.pdf', 'pdf', 320000, 'task_2/s3/v1.pdf', 'tcp_a1', 'parsed', 1, datetime('now', '-5 days')),
(2, 4, 'Wireshark_王小明.pdf', 'pdf', 280000, 'task_2/s4/v1.pdf', 'tcp_a2', 'parsed', 1, datetime('now', '-4 days')),
(2, 6, 'TCP三次握手_刘思琪.pdf', 'pdf', 245000, 'task_2/s6/v1.pdf', 'tcp_a3', 'parsed', 1, datetime('now', '-4 days')),
(2, 7, '协议分析_孙浩然.pdf', 'pdf', 310000, 'task_2/s7/v1.pdf', 'tcp_a4', 'parsed', 1, datetime('now', '-3 days')),
(2, 8, 'TCP_周雨萱.pdf', 'pdf', 198000, 'task_2/s8/v1.pdf', 'tcp_a5', 'parsed', 1, datetime('now', '-3 days')),
(2, 9, '网络协议_吴子轩.pdf', 'pdf', 267000, 'task_2/s9/v1.pdf', 'tcp_a6', 'parsed', 1, datetime('now', '-2 days')),
(2, 10, '抓包分析_郑雅琪.pdf', 'pdf', 224000, 'task_2/s10/v1.pdf', 'tcp_a7', 'parsed', 1, datetime('now', '-1 day')),
(2, 11, 'TCP实验_黄俊杰.pdf', 'pdf', 289000, 'task_2/s11/v1.pdf', 'tcp_a8', 'parsed', 1, datetime('now', '-1 day'));

-- Task 5 (RESTful API, closed) - 10 submissions
INSERT OR IGNORE INTO uploads (task_id, student_id, filename, file_type, file_size, storage_path, sha256, parse_status, version, created_at) VALUES
(5, 3, 'api_design_李同学.yaml', 'yaml', 45000, 'task_5/s3/v1.yaml', 'rest_a1', 'parsed', 1, datetime('now', '-14 days')),
(5, 4, 'restful_王小明.pdf', 'pdf', 230000, 'task_5/s4/v1.pdf', 'rest_a2', 'parsed', 1, datetime('now', '-14 days')),
(5, 5, 'swagger_赵文博.yaml', 'yaml', 52000, 'task_5/s5/v1.yaml', 'rest_a3', 'parsed', 1, datetime('now', '-13 days')),
(5, 6, 'api_刘思琪.pdf', 'pdf', 198000, 'task_5/s6/v1.pdf', 'rest_a4', 'parsed', 1, datetime('now', '-13 days')),
(5, 7, 'book_api_孙浩然.yaml', 'yaml', 38000, 'task_5/s7/v1.yaml', 'rest_a5', 'parsed', 1, datetime('now', '-12 days')),
(5, 8, 'restful_周雨萱.pdf', 'pdf', 215000, 'task_5/s8/v1.pdf', 'rest_a6', 'parsed', 1, datetime('now', '-12 days')),
(5, 9, 'api设计_吴子轩.yaml', 'yaml', 41000, 'task_5/s9/v1.yaml', 'rest_a7', 'parsed', 1, datetime('now', '-11 days')),
(5, 10, 'openapi_郑雅琪.yaml', 'yaml', 48000, 'task_5/s10/v1.yaml', 'rest_a8', 'parsed', 1, datetime('now', '-11 days')),
(5, 11, 'API文档_黄俊杰.pdf', 'pdf', 267000, 'task_5/s11/v1.pdf', 'rest_a9', 'parsed', 1, datetime('now', '-10 days')),
(5, 12, 'RESTful_林诗涵.yaml', 'yaml', 55000, 'task_5/s12/v1.yaml', 'rest_a10', 'parsed', 1, datetime('now', '-10 days'));

-- Task 6 (排序算法, closed) - 10 submissions
INSERT OR IGNORE INTO uploads (task_id, student_id, filename, file_type, file_size, storage_path, sha256, parse_status, version, created_at) VALUES
(6, 3, '排序对比_李同学.zip', 'zip', 156000, 'task_6/s3/v1.zip', 'sort_a1', 'parsed', 1, datetime('now', '-18 days')),
(6, 4, '快排归并_王小明.zip', 'zip', 189000, 'task_6/s4/v1.zip', 'sort_a2', 'parsed', 1, datetime('now', '-18 days')),
(6, 5, '排序算法_赵文博.pdf', 'pdf', 234000, 'task_6/s5/v1.pdf', 'sort_a3', 'parsed', 1, datetime('now', '-17 days')),
(6, 6, '堆排序_刘思琪.zip', 'zip', 145000, 'task_6/s6/v1.zip', 'sort_a4', 'parsed', 1, datetime('now', '-17 days')),
(6, 7, '排序benchmark_孙浩然.zip', 'zip', 267000, 'task_6/s7/v1.zip', 'sort_a5', 'parsed', 1, datetime('now', '-16 days')),
(6, 8, '排序实验_周雨萱.pdf', 'pdf', 198000, 'task_6/s8/v1.pdf', 'sort_a6', 'parsed', 1, datetime('now', '-16 days')),
(6, 9, '归并排序_吴子轩.zip', 'zip', 178000, 'task_6/s9/v1.zip', 'sort_a7', 'parsed', 1, datetime('now', '-15 days')),
(6, 10, '快排优化_郑雅琪.zip', 'zip', 212000, 'task_6/s10/v1.zip', 'sort_a8', 'parsed', 1, datetime('now', '-15 days')),
(6, 11, '排序_黄俊杰.zip', 'zip', 167000, 'task_6/s11/v1.zip', 'sort_a9', 'parsed', 1, datetime('now', '-14 days')),
(6, 12, '排序对比_林诗涵.pdf', 'pdf', 234000, 'task_6/s12/v1.pdf', 'sort_a10', 'parsed', 1, datetime('now', '-14 days'));

-- ============================================================
-- 评价数据（让批改进度和得分好看）
-- ============================================================
-- Task 5: 全部 confirmed（已关闭任务）
INSERT OR IGNORE INTO evaluations (task_id, student_id, upload_id, status, total_score, created_at) VALUES
(5, 3, 21, 'confirmed', 88.5, datetime('now', '-9 days')),
(5, 4, 22, 'confirmed', 76.2, datetime('now', '-9 days')),
(5, 5, 23, 'confirmed', 92.0, datetime('now', '-8 days')),
(5, 6, 24, 'confirmed', 81.3, datetime('now', '-8 days')),
(5, 7, 25, 'confirmed', 69.8, datetime('now', '-7 days')),
(5, 8, 26, 'confirmed', 85.7, datetime('now', '-7 days')),
(5, 9, 27, 'confirmed', 78.4, datetime('now', '-6 days')),
(5, 10, 28, 'confirmed', 90.1, datetime('now', '-6 days')),
(5, 11, 29, 'confirmed', 83.6, datetime('now', '-5 days')),
(5, 12, 30, 'confirmed', 87.2, datetime('now', '-5 days'));

-- Task 6: 全部 confirmed（已关闭任务）
INSERT OR IGNORE INTO evaluations (task_id, student_id, upload_id, status, total_score, created_at) VALUES
(6, 3, 31, 'confirmed', 82.6, datetime('now', '-12 days')),
(6, 4, 32, 'confirmed', 74.3, datetime('now', '-12 days')),
(6, 5, 33, 'confirmed', 88.9, datetime('now', '-11 days')),
(6, 6, 34, 'confirmed', 71.5, datetime('now', '-11 days')),
(6, 7, 35, 'confirmed', 93.2, datetime('now', '-10 days')),
(6, 8, 36, 'confirmed', 79.8, datetime('now', '-10 days')),
(6, 9, 37, 'confirmed', 86.1, datetime('now', '-9 days')),
(6, 10, 38, 'confirmed', 91.4, datetime('now', '-9 days')),
(6, 11, 39, 'confirmed', 77.3, datetime('now', '-8 days')),
(6, 12, 40, 'confirmed', 84.7, datetime('now', '-8 days'));

-- Task 1: 混合状态（scored + confirmed），让批改工作台有内容
INSERT OR IGNORE INTO evaluations (task_id, student_id, upload_id, status, total_score, created_at) VALUES
(1, 3, 1, 'confirmed', 85.3, datetime('now', '-4 days')),
(1, 4, 2, 'confirmed', 72.1, datetime('now', '-4 days')),
(1, 5, 3, 'confirmed', 91.4, datetime('now', '-3 days')),
(1, 6, 4, 'scored', 78.6, datetime('now', '-3 days')),
(1, 7, 5, 'scored', 83.2, datetime('now', '-2 days')),
(1, 8, 6, 'scored', 67.9, datetime('now', '-2 days')),
(1, 9, 7, 'scored', 80.5, datetime('now', '-1 day')),
(1, 10, 8, 'scored', 88.0, datetime('now', '-1 day')),
(1, 11, 9, 'scored', 76.8, datetime('now')),
(1, 12, 10, 'scored', 82.4, datetime('now')),
(1, 13, 11, 'scored', 79.1, datetime('now')),
(1, 14, 12, 'scored', 84.7, datetime('now'));

-- Task 2: 部分已评分
INSERT OR IGNORE INTO evaluations (task_id, student_id, upload_id, status, total_score, created_at) VALUES
(2, 3, 13, 'confirmed', 87.5, datetime('now', '-2 days')),
(2, 4, 14, 'confirmed', 79.3, datetime('now', '-2 days')),
(2, 6, 15, 'scored', 82.1, datetime('now', '-1 day')),
(2, 7, 16, 'scored', 75.8, datetime('now', '-1 day')),
(2, 8, 17, 'scored', 90.2, datetime('now')),
(2, 9, 18, 'scored', 71.6, datetime('now'));

-- ============================================================
-- 维度评分（Task 1 + Task 5 核心数据）
-- ============================================================
-- Task 5 dims: API设计合理性, 文档完整性, 规范遵循
INSERT OR IGNORE INTO dimension_scores (evaluation_id, dimension_id, ai_score, teacher_score, rationale) VALUES
(1, 17, 90.0, 88.0, 'API 路径设计清晰，RESTful 风格一致'),
(1, 18, 85.0, 87.0, '文档覆盖了主要接口，缺少错误码说明'),
(1, 19, 92.0, 91.0, '严格遵循 OpenAPI 3.0 规范'),
(2, 17, 72.0, 75.0, '部分接口命名不够直观'),
(2, 18, 78.0, 77.0, '文档结构完整但描述偏简略'),
(2, 19, 80.0, 76.0, '基本遵循规范，个别字段类型不准确'),
(3, 17, 95.0, 93.0, '设计优秀，资源划分合理'),
(3, 18, 90.0, 91.0, '文档详尽，含示例请求'),
(3, 19, 91.0, 92.0, '完全符合规范');

-- Task 1 dims: 代码规范性(1), 功能完整性(2), 并发正确性(3), 文档质量(4)
INSERT OR IGNORE INTO dimension_scores (evaluation_id, dimension_id, ai_score, teacher_score, rationale) VALUES
(21, 1, 82.0, 85.0, '命名规范，注释充分，部分方法可拆分'),
(21, 2, 88.0, 87.0, '生产者消费者模型完整实现，含缓冲区满/空处理'),
(21, 3, 85.0, 84.0, '使用 synchronized 正确，无死锁风险'),
(21, 4, 80.0, 82.0, '报告结构清晰，图表丰富'),
(22, 1, 68.0, 70.0, '变量命名不够规范，缺少注释'),
(22, 2, 75.0, 73.0, '基本功能实现，缺少异常处理'),
(22, 3, 70.0, 72.0, '存在潜在竞态条件'),
(22, 4, 72.0, 71.0, '报告过于简略'),
(23, 1, 92.0, 93.0, '代码风格优秀，模块化设计'),
(23, 2, 95.0, 94.0, '功能完整，含多种同步策略对比'),
(23, 3, 90.0, 91.0, '并发安全，使用 ReentrantLock'),
(23, 4, 85.0, 86.0, '报告详尽，含 UML 图');

-- ============================================================
-- 通知（丰富教师+学生+管理员，有已读未读）
-- ============================================================
INSERT INTO notifications (user_id, type, title, content, is_read, created_at) VALUES
-- 教师 teacher1 (id=2) 通知
(2, 'EVALUATION_COMPLETED', '批量评价完成：并发编程实训', '已完成 12 份 AI 自动评分，3 份已确认', 0, datetime('now', '-30 minutes')),
(2, 'SIMILARITY_DETECTED', '检测到疑似相似度过高的提交', '并发编程实训中 2 份提交相似度 89%，建议人工复核', 0, datetime('now', '-2 hours')),
(2, 'TASK_PUBLISHED', '你发布了新任务：TCP 协议分析', '已通知 27 名学生', 1, datetime('now', '-1 day')),
(2, 'EVALUATION_COMPLETED', '批量评价完成：TCP 协议分析', '已完成 6 份 AI 评分', 0, datetime('now', '-4 hours')),
(2, 'DEADLINE_APPROACHING', '任务即将截止：并发编程实训', '距离截止还有 3 天，尚有 3 名学生未提交', 0, datetime('now', '-6 hours')),
(2, 'EVALUATION_COMPLETED', 'RESTful API 设计评价全部确认', '10 份评价已全部确认，班级平均分 83.3', 1, datetime('now', '-5 days')),
(2, 'TASK_PUBLISHED', '你发布了新任务：并发编程实训', '已通知 35 名学生', 1, datetime('now', '-7 days')),
-- 学生 student1 (id=3) 通知
(3, 'TASK_PUBLISHED', '新任务已发布：并发编程实训', '请在截止时间前提交你的实训成果', 0, datetime('now', '-1 hour')),
(3, 'EVALUATION_COMPLETED', '评价已生成：并发编程实训', '你的综合得分为 85.3，查看详细报告', 0, datetime('now', '-3 days')),
(3, 'DEADLINE_APPROACHING', '任务即将截止：TCP 协议分析', '距离截止还有 7 天，请尽快提交', 0, datetime('now', '-2 hours')),
(3, 'EVALUATION_COMPLETED', '评价已生成：RESTful API 设计', '你的综合得分为 88.5，查看详细报告', 1, datetime('now', '-9 days')),
(3, 'EVALUATION_COMPLETED', '评价已生成：排序算法对比', '你的综合得分为 82.6，查看详细报告', 1, datetime('now', '-12 days')),
(3, 'SIMILARITY_DETECTED', '提交相似度提醒', '系统检测到你的提交与同班同学存在较高相似度', 0, datetime('now', '-5 hours')),
-- 管理员 admin (id=1) 通知
(1, 'EVALUATION_COMPLETED', '系统评价统计', '本周共完成 38 份自动评价，确认率 68%', 0, datetime('now', '-3 hours')),
(1, 'TASK_PUBLISHED', '教师发布了新任务', '张老师发布了"TCP 协议分析"', 1, datetime('now', '-1 day')),
(1, 'EVALUATION_COMPLETED', '月度评价报告', '本月累计完成 156 份评价，系统运行正常', 1, datetime('now', '-3 days'));

-- ============================================================
-- 审计日志（近7天丰富数据）
-- ============================================================
INSERT INTO audit_logs (occurred_at, user_id, username, role, action, target_type, target_id, result, detail, client_ip) VALUES
(datetime('now', '-5 minutes'), 2, 'teacher1', 'teacher', 'auth.login', 'user', '2', 'success', '教师登录系统', '192.168.1.101'),
(datetime('now', '-15 minutes'), 3, 'student1', 'student', 'upload.created', 'upload', '13', 'success', '提交 TCP 分析报告', '192.168.1.102'),
(datetime('now', '-30 minutes'), 2, 'teacher1', 'teacher', 'evaluation.confirmed', 'evaluation', '21', 'success', '确认评价：并发编程实训 - 李同学', '192.168.1.101'),
(datetime('now', '-1 hour'), 2, 'teacher1', 'teacher', 'evaluation.confirmed', 'evaluation', '22', 'success', '确认评价：并发编程实训 - 王小明', '192.168.1.101'),
(datetime('now', '-2 hours'), 1, 'admin', 'admin', 'auth.login', 'user', '1', 'success', '管理员登录', '192.168.1.100'),
(datetime('now', '-3 hours'), 2, 'teacher1', 'teacher', 'evaluation.auto_scored', 'evaluation', '26', 'success', 'AI 自动评分完成 6 份', '192.168.1.101'),
(datetime('now', '-4 hours'), 4, 'student2', 'student', 'upload.created', 'upload', '14', 'success', '提交 Wireshark 抓包分析', '192.168.1.103'),
(datetime('now', '-5 hours'), 2, 'teacher1', 'teacher', 'task.published', 'task', '2', 'success', '发布任务：TCP 协议分析', '192.168.1.101'),
(datetime('now', '-8 hours'), 3, 'student1', 'student', 'auth.login', 'user', '3', 'success', '学生登录', '192.168.1.102'),
(datetime('now', '-1 day'), 2, 'teacher1', 'teacher', 'evaluation.confirmed', 'evaluation', '23', 'success', '确认评价：并发编程 - 赵文博', '192.168.1.101'),
(datetime('now', '-1 day'), 5, 'student3', 'student', 'upload.created', 'upload', '3', 'success', '提交并发实训源码', '192.168.1.104'),
(datetime('now', '-2 days'), 2, 'teacher1', 'teacher', 'task.closed', 'task', '5', 'success', '关闭任务：RESTful API 设计', '192.168.1.101'),
(datetime('now', '-2 days'), 1, 'admin', 'admin', 'llm.config.updated', 'llm_config', '1', 'success', '更新 LLM 配置：切换为 DeepSeek V3', '192.168.1.100'),
(datetime('now', '-3 days'), 2, 'teacher1', 'teacher', 'evaluation.auto_scored', 'evaluation', '1', 'success', '批量 AI 评分完成：RESTful API 设计', '192.168.1.101'),
(datetime('now', '-3 days'), 3, 'student1', 'student', 'chat.message', 'session', '1', 'success', 'AI 问答：如何优化并发代码', '192.168.1.102'),
(datetime('now', '-4 days'), 1, 'admin', 'admin', 'user.created', 'user', '20', 'success', '批量导入 5 名学生', '192.168.1.100'),
(datetime('now', '-5 days'), 2, 'teacher1', 'teacher', 'task.published', 'task', '1', 'success', '发布任务：并发编程实训', '192.168.1.101'),
(datetime('now', '-5 days'), 1, 'admin', 'admin', 'auth.login.failed', 'user', '1', 'failed', '密码错误（1/5）', '10.0.0.55'),
(datetime('now', '-6 days'), 6, 'student4', 'student', 'upload.created', 'upload', '4', 'success', '提交线程同步实验', '192.168.1.105'),
(datetime('now', '-6 days'), 2, 'teacher1', 'teacher', 'auth.login', 'user', '2', 'success', '教师登录', '192.168.1.101');

-- ============================================================
-- 相似度记录（让教师仪表盘有警告数据）
-- ============================================================
INSERT OR IGNORE INTO similarity_records (task_id, upload_a_id, upload_b_id, hamming_distance, cosine_similarity, state) VALUES
(1, 1, 4, 5, 0.89, 'suspect'),
(1, 5, 6, 7, 0.86, 'suspect'),
(1, 9, 10, 12, 0.72, 'ignored');

-- ============================================================
-- 评价模板
-- ============================================================
INSERT OR IGNORE INTO eval_templates (name, description, visibility, owner_id) VALUES
('软件工程通用模板', '适用于软件工程类实训的通用评价维度', 'system', NULL),
('算法实现模板', '适用于算法类实训任务', 'system', NULL),
('张老师自定义模板', '并发编程专用评价标准', 'private', 2);

INSERT OR IGNORE INTO template_dimensions (template_id, name, weight, order_index) VALUES
(1, '代码规范性', 25, 0),(1, '功能完整性', 35, 1),(1, '测试覆盖', 20, 2),(1, '文档质量', 20, 3),
(2, '算法正确性', 40, 0),(2, '时间复杂度', 30, 1),(2, '代码风格', 15, 2),(2, '分析报告', 15, 3),
(3, '并发安全性', 30, 0),(3, '性能表现', 25, 1),(3, '代码结构', 25, 2),(3, '实验报告', 20, 3);

-- ============================================================
-- 学生画像（student1 = id 3）
-- ============================================================
INSERT OR REPLACE INTO student_profiles (student_id, radar_data, weakness_list, suggestions, score_trend, source_evaluation_count, computed_at) VALUES
(3,
 '{"代码规范性": 85, "功能完整性": 88, "并发正确性": 82, "文档质量": 76, "API设计": 90, "算法实现": 83}',
 '[{"name": "文档质量", "score": 76}, {"name": "并发正确性", "score": 82}, {"name": "算法实现", "score": 83}]',
 '["建议在实验报告中增加性能测试数据和对比分析", "复习 Java 并发包中 Lock 与 synchronized 的区别", "注意方法长度控制，单个方法不超过 30 行"]',
 '[{"period": "T1", "score": 72.5}, {"period": "T2", "score": 78.3}, {"period": "T3", "score": 82.6}, {"period": "T4", "score": 85.3}, {"period": "T5", "score": 88.5}]',
 5, datetime('now', '-1 hour'));

-- ============================================================
-- Chat sessions（让学生 AI 问答页有内容）
-- ============================================================
INSERT OR IGNORE INTO chat_sessions (student_id, title, created_at, last_active_at) VALUES
(3, '关于并发编程的问题', datetime('now', '-2 days'), datetime('now', '-2 days')),
(3, '如何写好实训报告', datetime('now', '-5 days'), datetime('now', '-4 days'));

INSERT OR IGNORE INTO chat_messages (session_id, role, content, created_at) VALUES
(1, 'user', '请问 Java 中 synchronized 和 ReentrantLock 有什么区别？', datetime('now', '-2 days')),
(1, 'assistant', 'synchronized 是 Java 内置的关键字，使用简单但灵活性较低。ReentrantLock 是 java.util.concurrent 包中的类，提供了更多功能：1) 可中断锁等待；2) 公平锁选择；3) 多条件变量支持；4) tryLock 超时机制。在简单场景用 synchronized 即可，复杂并发控制建议用 ReentrantLock。', datetime('now', '-2 days')),
(1, 'user', '那我的生产者消费者模型应该用哪个？', datetime('now', '-2 days')),
(1, 'assistant', '对于生产者消费者模型，两种都可以。如果你需要精确控制"缓冲区满"和"缓冲区空"两个条件，ReentrantLock + Condition 更优雅（可以分别创建 notFull 和 notEmpty 条件）。如果追求代码简洁，synchronized + wait/notifyAll 也完全可行。建议你在报告中对比两种实现的优劣。', datetime('now', '-2 days')),
(2, 'user', '实训报告应该包含哪些部分？', datetime('now', '-5 days')),
(2, 'assistant', '一份优秀的实训报告通常包含：1) 项目概述与目标；2) 设计方案（含类图/流程图）；3) 核心代码实现与解释；4) 测试用例与运行结果；5) 性能分析（如有）；6) 遇到的问题与解决方案；7) 总结与收获。建议配合 UML 图和代码注释截图，让报告更直观。', datetime('now', '-5 days'));
