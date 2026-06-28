-- Idempotent data supplement for the demo database.
-- Safe to run multiple times: every statement guards against duplicates.
--
-- 1) Assign students that belong to no class (orphans) round-robin into the
--    classes that currently have zero members, so the teacher side no longer
--    shows empty classes.
-- 2) Seed a set of reusable evaluation templates (with dimensions), since the
--    template library shipped empty.

-- ── 1. fill empty classes with orphaned students ───────────────────────────
WITH orphans AS (
    SELECT u.id AS sid, ROW_NUMBER() OVER (ORDER BY u.id) AS rn
    FROM users u
    WHERE u.role = 'student'
      AND NOT EXISTS (SELECT 1 FROM class_memberships cm WHERE cm.student_id = u.id)
),
empty_classes AS (
    SELECT c.id AS cid, ROW_NUMBER() OVER (ORDER BY c.id) AS cn
    FROM classes c
    WHERE NOT EXISTS (SELECT 1 FROM class_memberships cm WHERE cm.class_id = c.id)
),
ec AS (SELECT COUNT(*) AS n FROM empty_classes)
INSERT INTO class_memberships (class_id, student_id)
SELECT e.cid, o.sid
FROM orphans o
JOIN ec
JOIN empty_classes e ON e.cn = ((o.rn - 1) % ec.n) + 1
WHERE ec.n > 0;

-- ── 2. evaluation templates ────────────────────────────────────────────────
-- owner_id 2 = teacher1; visibility 'system' so every teacher can reuse them.
INSERT INTO eval_templates (name, description, visibility, owner_id, course_id)
SELECT '实训通用评价模板', '适用于各类实训作业的通用四维度评价标准', 'system', 2, NULL
WHERE NOT EXISTS (SELECT 1 FROM eval_templates WHERE name = '实训通用评价模板');

INSERT INTO eval_templates (name, description, visibility, owner_id, course_id)
SELECT '程序设计实训评价', '面向编程类实训，侧重功能实现与代码质量', 'system', 2,
       (SELECT id FROM courses WHERE name = 'Python程序设计' ORDER BY id LIMIT 1)
WHERE NOT EXISTS (SELECT 1 FROM eval_templates WHERE name = '程序设计实训评价');

INSERT INTO eval_templates (name, description, visibility, owner_id, course_id)
SELECT '数据库实训评价', '面向数据库实训，侧重 SQL 正确性与表结构设计', 'system', 2,
       (SELECT id FROM courses WHERE name = '数据库系统' ORDER BY id LIMIT 1)
WHERE NOT EXISTS (SELECT 1 FROM eval_templates WHERE name = '数据库实训评价');

INSERT INTO eval_templates (name, description, visibility, owner_id, course_id)
SELECT '网络配置实训评价', '面向网络实训，侧重配置正确性与连通性测试', 'system', 2,
       (SELECT id FROM courses WHERE name = '计算机网络' ORDER BY id LIMIT 1)
WHERE NOT EXISTS (SELECT 1 FROM eval_templates WHERE name = '网络配置实训评价');

INSERT INTO eval_templates (name, description, visibility, owner_id, course_id)
SELECT '数据分析实训评价', '面向大数据分析实训，侧重数据处理与可视化呈现', 'system', 2,
       (SELECT id FROM courses WHERE name = '大数据技术' ORDER BY id LIMIT 1)
WHERE NOT EXISTS (SELECT 1 FROM eval_templates WHERE name = '数据分析实训评价');

-- template dimensions (guarded per template + name)
INSERT INTO template_dimensions (template_id, name, description, weight, order_index)
SELECT t.id, d.name, d.description, d.weight, d.order_index
FROM eval_templates t
JOIN (
    SELECT '实训通用评价模板' AS tpl, '命令操作正确性' AS name, '操作指令准确、无误操作' AS description, 30 AS weight, 1 AS order_index
    UNION ALL SELECT '实训通用评价模板', '操作过程完整性', '步骤完整、流程规范', 25, 2
    UNION ALL SELECT '实训通用评价模板', '结果记录规范性', '记录清晰、数据真实', 25, 3
    UNION ALL SELECT '实训通用评价模板', '实验总结质量', '总结到位、反思充分', 20, 4

    UNION ALL SELECT '程序设计实训评价', '功能实现', '需求覆盖完整、运行正确', 40, 1
    UNION ALL SELECT '程序设计实训评价', '代码规范', '命名清晰、结构合理', 20, 2
    UNION ALL SELECT '程序设计实训评价', '算法效率', '复杂度合理、无明显冗余', 20, 3
    UNION ALL SELECT '程序设计实训评价', '文档与注释', '注释充分、文档完整', 20, 4

    UNION ALL SELECT '数据库实训评价', 'SQL 正确性', '语句正确、结果符合预期', 35, 1
    UNION ALL SELECT '数据库实训评价', '表结构设计', '范式合理、约束完整', 25, 2
    UNION ALL SELECT '数据库实训评价', '查询优化', '索引合理、性能良好', 20, 3
    UNION ALL SELECT '数据库实训评价', '实验报告', '报告规范、分析清晰', 20, 4

    UNION ALL SELECT '网络配置实训评价', '配置正确性', '设备配置准确无误', 35, 1
    UNION ALL SELECT '网络配置实训评价', '连通性测试', '测试充分、结果可达', 25, 2
    UNION ALL SELECT '网络配置实训评价', '故障排查', '定位准确、处理得当', 20, 3
    UNION ALL SELECT '网络配置实训评价', '总结规范', '记录规范、总结到位', 20, 4

    UNION ALL SELECT '数据分析实训评价', '数据预处理', '清洗合理、特征得当', 25, 1
    UNION ALL SELECT '数据分析实训评价', '分析方法', '方法合适、逻辑严谨', 25, 2
    UNION ALL SELECT '数据分析实训评价', '可视化效果', '图表清晰、表达准确', 25, 3
    UNION ALL SELECT '数据分析实训评价', '结论与报告', '结论可靠、报告规范', 25, 4
) d ON d.tpl = t.name
WHERE NOT EXISTS (
    SELECT 1 FROM template_dimensions td WHERE td.template_id = t.id AND td.name = d.name
);
