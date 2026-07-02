const s = 'width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"'

export const dagNodeIcons: Record<string, string> = {
  // 发射台 - 启动/初始化
  setup: `<svg ${s}><path d="M12 2L8 10h8L12 2z"/><path d="M6 10h12"/><path d="M8 10v4"/><path d="M16 10v4"/><rect x="5" y="14" width="14" height="3" rx="1"/><path d="M9 17v3"/><path d="M15 17v3"/><rect x="7" y="20" width="10" height="2" rx="1"/></svg>`,

  // 链接 - HTTP请求
  http: `<svg ${s}><path d="M10 13a5 5 0 007.54.54l3-3a5 5 0 00-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 00-7.54-.54l-3 3a5 5 0 007.07 7.07l1.71-1.71"/></svg>`,

  // 沙漏 - 延迟等待
  delay: `<svg ${s}><path d="M5 2h14v4l-5 6 5 6v4H5v-4l5-6-5-6V2z"/><path d="M8 2v3"/><path d="M16 2v3"/><path d="M8 19v3"/><path d="M16 19v3"/><path d="M12 10v4" stroke-dasharray="1 2"/></svg>`,

  // 决策树 - 条件判断
  condition: `<svg ${s}><circle cx="12" cy="4" r="2" fill="currentColor"/><path d="M12 6v4"/><path d="M8 10h8"/><path d="M8 10v4"/><path d="M16 10v4"/><circle cx="8" cy="16" r="2" fill="currentColor"/><circle cx="16" cy="16" r="2" fill="currentColor"/></svg>`,

  // 分路箭头 - 分支逻辑
  'if-else': `<svg ${s}><path d="M12 20V12"/><path d="M12 12l-7-7"/><path d="M12 12l7-7"/><path d="M5 5l2 0 0 2"/><path d="M19 5l-2 0 0 2"/></svg>`,

  // 垃圾桶 - 清理/结束
  teardown: `<svg ${s}><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/><line x1="10" y1="11" x2="10" y2="17"/><line x1="14" y1="11" x2="14" y2="17"/></svg>`,

  // 堆叠 - 分组容器
  group: `<svg ${s}><path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 12l10 5 10-5"/><path d="M2 17l10 5 10-5"/></svg>`,

  // 闹钟 - 定时触发
  timer: `<svg ${s}><circle cx="12" cy="13" r="8"/><path d="M12 9v4l2 2"/><path d="M5 3L2 6"/><path d="M22 6l-3-3"/><path d="M6 19l-2 2"/><path d="M18 19l2 2"/></svg>`,

  // 重复 - 条件循环
  while: `<svg ${s}><path d="M17 2l4 4-4 4"/><path d="M3 11V9a4 4 0 014-4h14"/><path d="M7 22l-4-4 4-4"/><path d="M21 13v2a4 4 0 01-4 4H3"/></svg>`,

  // 双轨 - 并行执行
  parallel: `<svg ${s}><path d="M4 6h16"/><path d="M4 12h16"/><path d="M4 18h16"/><circle cx="4" cy="6" r="1.5" fill="currentColor"/><circle cx="4" cy="12" r="1.5" fill="currentColor"/><circle cx="4" cy="18" r="1.5" fill="currentColor"/><circle cx="20" cy="6" r="1.5" fill="currentColor"/><circle cx="20" cy="12" r="1.5" fill="currentColor"/><circle cx="20" cy="18" r="1.5" fill="currentColor"/></svg>`,

  // 调用栈 - 子流程
  sub_flow: `<svg ${s}><rect x="4" y="2" width="16" height="5" rx="1"/><rect x="7" y="9" width="13" height="5" rx="1"/><rect x="10" y="16" width="10" height="5" rx="1"/><path d="M12 7v2"/><path d="M13.5 14v2"/></svg>`,

  // 重复N次 - 固定循环
  loop: `<svg ${s}><path d="M21 2v6h-6"/><path d="M3 12a9 9 0 0115-6.7L21 8"/><path d="M21 22v-6h-6"/><path d="M3 12a9 9 0 0015 6.7L21 16"/><text x="12" y="14" font-size="5" fill="currentColor" text-anchor="middle" font-weight="700">N</text></svg>`,

  // 数据库 - 数据生成
  generator: `<svg ${s}><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 5v6c0 1.66-4.03 3-9 3s-9-1.34-9-3V5"/><path d="M21 11v6c0 1.66-4.03 3-9 3s-9-1.34-9-3v-6"/><path d="M21 17v2c0 1.66-4.03 3-9 3s-9-1.34-9-3v-2"/></svg>`,
}

export function getDagIcon(type: string): string {
  return dagNodeIcons[type] || dagNodeIcons['generator']!
}
