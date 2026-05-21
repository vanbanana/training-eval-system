"""测试 Factory 注册中心.

每个 Epic 在此模块下补充对应 Factory：
- UserFactory / TeacherFactory / AdminFactory（Epic 3）
- CourseFactory / ClassFactory（Epic 4）
- TrainingTaskFactory / DimensionFactory（Epic 5）
- EvaluationTemplateFactory（Epic 6）
- UploadFactory（Epic 8）
- EvaluationFactory（Epic 16）
- LLMResponseFactory（Epic 11）

设计要求：
- 全部基于 factory_boy + Faker
- 默认值产出 valid 实例
- 字段完全可 override
- 不写死字面值（含 Faker locale=zh_CN）
"""

from __future__ import annotations

from faker import Faker

faker = Faker("zh_CN")
faker.seed_instance(20260519)  # 测试可重现


__all__ = ["faker"]
