## ADDED Requirements

### Requirement: 每个阿里图片内容项均转换为独立结果
系统 SHALL 将阿里同步图片响应中每个非空 `choice.message.content[].image` 转换为一个独立的 OpenAI 图片数据项，并保持 choice 与 content 的图片顺序。

#### Scenario: 单个 choice 返回多张图片
- **WHEN** 一个 choice 的 content 包含两张图片
- **THEN** 标准图片响应包含两个按原顺序排列的数据项

#### Scenario: 多个 choice 返回图片
- **WHEN** 多个 choice 分别包含一张或多张图片
- **THEN** 标准图片响应按 choice 顺序和各自 content 顺序包含全部图片

### Requirement: 修订提示附加到同一 choice 的全部图片
系统 SHALL 将 choice 中最后一个非空文本内容作为该 choice 每张图片的 `revised_prompt`，且 SHALL 不受文本位于图片之前或之后的影响。

#### Scenario: 文本位于多张图片之后
- **WHEN** 一个 choice 先返回两张图片，随后返回修订提示文本
- **THEN** 两个图片数据项均包含相同的修订提示

### Requirement: 不生成空图片项
系统 SHALL 忽略不含有效图片内容的 choice，文本内容不得单独生成 OpenAI 图片数据项。

#### Scenario: choice 仅包含文本或空内容
- **WHEN** 一个 choice 不包含非空图片内容
- **THEN** 该 choice 不增加标准图片响应的数据项
