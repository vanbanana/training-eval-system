# 06 LLM Skills Catalog

**Skill = 一个具体的 LLM 任务单元**。每个 Skill 是 prompt 模板、输入 schema、输出 schema、可选工具集、评估集（golden set）的统一封装。Skill 是本系统所有 LLM 调用的唯一入口。

## Skill 抽象定义

```python
# app/llm/skills/base.py
from abc import ABC, abstractmethod
from typing import Generic, TypeVar
from pydantic import BaseModel

InputT = TypeVar("InputT", bound=BaseModel)
OutputT = TypeVar("OutputT", bound=BaseModel)

class Skill(ABC, Generic[InputT, OutputT]):
    name: str                    # 唯一标识，如 "score.dimension"
    version: str                 # 语义化版本，如 "1.2.0"
    category: str                # parse | verify | score | profile | chat
    input_schema: type[InputT]
    output_schema: type[OutputT]
    tools: list["Tool"] = []     # 可选 Function Calling 工具
    temperature: float = 0.2
    max_tokens: int = 1500
    
    @abstractmethod
    def render_prompt(self, input_: InputT) -> list[LLMMessage]:
        """根据输入渲染 system + user 消息"""
        ...
    
    @abstractmethod
    def parse_output(self, raw: str) -> OutputT:
        """从 LLM 原始输出解析为结构化 schema，失败抛 SkillOutputError"""
        ...

    async def execute(self, input_: InputT, llm: LLMProvider) -> OutputT:
        """统一执行入口：渲染→调用→解析→重试→记录"""
        ...
```

## Skill 分类

| 分类 | 用途 | 典型 Skill |
|------|------|-----------|
| **parse** | 文档结构化解析 | docx_to_structure / pdf_to_structure / image_to_text |
| **verify** | 实训成果核查 | coverage_check / logic_audit |
| **score** | 评分生成 | dimension_score |
| **profile** | 画像与建议 | weakness_analyze / learning_advice / teaching_summary |
| **chat** | 多轮交互 | student_qa（含工具集） |

## Skill 注册中心

```python
# app/llm/skills/registry.py
class SkillRegistry:
    """全局 Skill 单例注册中心"""
    _skills: dict[str, Skill] = {}
    
    @classmethod
    def register(cls, skill: Skill) -> None:
        key = f"{skill.name}@{skill.version}"
        cls._skills[key] = skill
    
    @classmethod
    def get(cls, name: str, version: str | None = None) -> Skill:
        """version 为 None 时返回最新版"""
        ...
    
    @classmethod
    def list_by_category(cls, category: str) -> list[Skill]:
        ...
```

应用启动时通过 `app/llm/skills/__init__.py` 自动导入并注册所有 Skill 子类。

## 版本化与 Golden Set 评估

- 每个 Skill 必须维护 `tests/skills/golden/{name}/` 目录，存放 `input.json` 和 `expected.json`
- 升级 Skill 版本时必须运行 `tes-cli skill-eval --skill=score.dimension --version=1.2.0` 验证回归
- 业务代码引用 Skill 时**必须指定版本**：`SkillRegistry.get("score.dimension", "1.2.0")`，避免静默升级影响行为
- 提示词模板存放在独立的 Jinja2 文件中（如 `app/llm/skills/score/dimension_score.j2`），不嵌入 Python 代码

## Skill 命名规约

- 格式：`<category>.<action>[.<sub>]`
- 全部小写，用 `.` 分层，用 `_` 连接单词
- 示例：`parse.docx`、`score.dimension`、`profile.weakness_analyze`

## 单一 Skill 调用示例

```python
class DimensionScoreInput(BaseModel):
    task_requirements: str
    dimension_name: str
    dimension_description: str
    parse_summary: str
    verify_report: VerifyResultData

class DimensionScoreOutput(BaseModel):
    score: int = Field(ge=0, le=100)
    rationale: str = Field(min_length=50, max_length=200)

class DimensionScoreSkill(Skill[DimensionScoreInput, DimensionScoreOutput]):
    name = "score.dimension"
    version = "1.0.0"
    category = "score"
    input_schema = DimensionScoreInput
    output_schema = DimensionScoreOutput
    
    def render_prompt(self, inp):
        sys = render_template("score/dimension_score.j2", **inp.model_dump())
        return [LLMMessage(role="system", content=sys)]
    
    def parse_output(self, raw):
        try:
            return DimensionScoreOutput.model_validate_json(extract_json_block(raw))
        except ValidationError as e:
            raise SkillOutputError("score.dimension", raw, e)

# 业务侧调用
skill = SkillRegistry.get("score.dimension", "1.0.0")
result = await skill.execute(DimensionScoreInput(...), llm=LLMFactory.current())
```

## 当前规划的 Skill 清单

| Skill | 版本 | 分类 | 用途 |
|-------|------|------|------|
| `parse.docx` | 1.0.0 | parse | Word 文档结构化提取 |
| `parse.pdf` | 1.0.0 | parse | PDF 文档结构化提取 |
| `parse.image` | 1.0.0 | parse | OCR 后文本理解 |
| `verify.coverage_check` | 1.0.0 | verify | 实训要求覆盖度核查 |
| `verify.logic_audit` | 1.0.0 | verify | 逻辑漏洞识别 |
| `score.dimension` | 1.0.0 | score | 单维度客观评分 |
| `profile.weakness_analyze` | 1.0.0 | profile | 学生薄弱点分析 |
| `profile.learning_advice` | 1.0.0 | profile | 学习建议生成 |
| `profile.teaching_summary` | 1.0.0 | profile | 教学画像总结 |
| `chat.student_qa` | 1.0.0 | chat | 学生问答（含工具调用） |

## 与 Tool 的关系

- 多轮工具调用场景（chat 类）→ Skill 内 `tools` 非空，`execute()` 走工具调用循环
- 单轮无工具场景（其他所有类）→ Skill 内 `tools` 为空，普通一次性调用
- **Tool ≠ Skill**：Tool 是给 LLM 在运行时调用的"动作"，Skill 是工程师定义的"任务"
- 所有 Skill 与 Tool 共用同一 LLMProvider 和 trace_id，便于统一观测

详见 [07 Function Calling 工具](07-function-calling-tools.md)。

## 提示词工程指南

- 系统提示词放 `system` 角色，规则与背景；用户输入放 `user` 角色
- 输出格式必须显式约束（如"严格输出 JSON：{...}"）
- 关键参数（实训要求、评分维度等）通过 Jinja2 变量插入，不要拼接字符串
- 输出 schema 在 `parse_output` 中用 Pydantic 验证，失败重试 3 次
- 复杂任务考虑分步：第一步生成中间结构，第二步基于中间结构生成最终输出
