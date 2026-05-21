/**
 * Markdown 渲染工具 — 基于 markdown-it + highlight.js
 *
 * 特性：
 * - 代码块语法高亮
 * - 安全 HTML 转义（防 XSS）
 * - 链接新窗口打开
 * - 表格支持
 */
import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js/lib/core'

// 按需注册常用语言（减小 bundle）
import python from 'highlight.js/lib/languages/python'
import javascript from 'highlight.js/lib/languages/javascript'
import typescript from 'highlight.js/lib/languages/typescript'
import java from 'highlight.js/lib/languages/java'
import cpp from 'highlight.js/lib/languages/cpp'
import bash from 'highlight.js/lib/languages/bash'
import json from 'highlight.js/lib/languages/json'
import sql from 'highlight.js/lib/languages/sql'
import xml from 'highlight.js/lib/languages/xml'
import css from 'highlight.js/lib/languages/css'

hljs.registerLanguage('python', python)
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('typescript', typescript)
hljs.registerLanguage('java', java)
hljs.registerLanguage('cpp', cpp)
hljs.registerLanguage('bash', bash)
hljs.registerLanguage('json', json)
hljs.registerLanguage('sql', sql)
hljs.registerLanguage('xml', xml)
hljs.registerLanguage('html', xml)
hljs.registerLanguage('css', css)

const md = new MarkdownIt({
  html: false, // 禁止原始 HTML（安全）
  linkify: true, // 自动识别链接
  typographer: true,
  highlight(str: string, lang: string): string {
    if (lang && hljs.getLanguage(lang)) {
      try {
        const result = hljs.highlight(str, { language: lang })
        return `<pre class="hljs-code-block"><code class="hljs language-${lang}">${result.value}</code></pre>`
      } catch {
        // fallthrough
      }
    }
    // 无语言或识别失败 — 纯文本 code block
    const escaped = md.utils.escapeHtml(str)
    return `<pre class="hljs-code-block"><code>${escaped}</code></pre>`
  },
})

// 链接新窗口打开
const defaultRender =
  md.renderer.rules.link_open ||
  function (tokens, idx, options, _env, self) {
    return self.renderToken(tokens, idx, options)
  }

md.renderer.rules.link_open = function (tokens, idx, options, env, self) {
  tokens[idx].attrSet('target', '_blank')
  tokens[idx].attrSet('rel', 'noopener noreferrer')
  return defaultRender(tokens, idx, options, env, self)
}

/**
 * 渲染 Markdown 为 HTML 字符串
 */
export function renderMarkdown(source: string): string {
  if (!source) return ''
  return md.render(source)
}

/**
 * 渲染行内 Markdown（不包裹 <p>）
 */
export function renderInline(source: string): string {
  if (!source) return ''
  return md.renderInline(source)
}
