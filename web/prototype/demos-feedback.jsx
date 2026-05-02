// Feedback / loading / confirm / validation-jump demo variants.

// =============================================================
// 2. Feedback — 保存成功/失败
// =============================================================

// 2a. Toast (success)
function DemoToastSuccess(props) {
  return (
    <ShellB {...props} page="sources">
      <div style={{ height: '100%', position: 'relative', overflow: 'hidden' }}>
        <SourcesBodyDemo />
        <div style={{
          position: 'absolute', right: 24, bottom: 24,
          background: B.panel, border: `1px solid ${B.border}`, borderRadius: 10,
          padding: '12px 16px', boxShadow: '0 12px 32px rgba(15,23,42,0.15)',
          display: 'flex', alignItems: 'center', gap: 12, minWidth: 320,
          borderLeft: '4px solid #22c55e',
        }}>
          <div style={{
            width: 28, height: 28, borderRadius: 14, background: '#dcfce7',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            color: '#16a34a', fontWeight: 700, fontSize: 14,
          }}>✓</div>
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 13, fontWeight: 600 }}>保存并热重载成功</div>
            <div style={{ fontSize: 11, color: B.textMuted, marginTop: 2 }}>142ms · 14 节点 · 6 分组生效</div>
          </div>
          <button style={iconBtnB()}>✕</button>
        </div>
      </div>
    </ShellB>
  );
}

// 2b. Toast (error)
function DemoToastError(props) {
  return (
    <ShellB {...props} page="sources">
      <div style={{ height: '100%', position: 'relative', overflow: 'hidden' }}>
        <SourcesBodyDemo />
        <div style={{
          position: 'absolute', right: 24, bottom: 24,
          background: B.panel, border: `1px solid ${B.border}`, borderRadius: 10,
          padding: '12px 16px', boxShadow: '0 12px 32px rgba(15,23,42,0.15)',
          display: 'flex', alignItems: 'flex-start', gap: 12, minWidth: 380,
          borderLeft: '4px solid #ef4444',
        }}>
          <div style={{
            width: 28, height: 28, borderRadius: 14, background: '#fee2e2',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            color: '#dc2626', fontWeight: 700, fontSize: 14, flex: '0 0 auto',
          }}>!</div>
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 13, fontWeight: 600 }}>校验失败</div>
            <div style={{ fontSize: 11, color: B.textMuted, marginTop: 2, lineHeight: 1.5 }}>
              发现 3 个错误，请前往「配置校验」页面查看详情
            </div>
            <div style={{ display: 'flex', gap: 12, marginTop: 8 }}>
              <a style={{ fontSize: 12, color: B.accent, fontWeight: 500, cursor: 'pointer' }}>查看详情 →</a>
              <a style={{ fontSize: 12, color: B.textMuted, cursor: 'pointer' }}>忽略</a>
            </div>
          </div>
          <button style={iconBtnB()}>✕</button>
        </div>
      </div>
    </ShellB>
  );
}

// 2c. Top banner (error)
function DemoBannerError(props) {
  return (
    <ShellB {...props} page="sources" topbar={
      <div>
        <TopbarB page="sources"/>
        <div style={{
          padding: '10px 32px', background: '#fef2f2', borderBottom: `1px solid #fecaca`,
          display: 'flex', alignItems: 'center', gap: 12,
        }}>
          <span style={{
            width: 22, height: 22, borderRadius: 11, background: '#dc2626',
            color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 13, fontWeight: 700, flex: '0 0 auto',
          }}>!</span>
          <div style={{ flex: 1 }}>
            <span style={{ fontSize: 13, fontWeight: 600, color: '#991b1b' }}>保存失败 · </span>
            <span style={{ fontSize: 13, color: '#7f1d1d' }}>检测到 3 个错误，2 个警告。修复后才能写回 config.yaml。</span>
          </div>
          <button style={{
            padding: '5px 12px', borderRadius: 6, border: '1px solid #fca5a5',
            background: '#fff', color: '#991b1b', fontSize: 12, fontWeight: 500, cursor: 'pointer',
            whiteSpace: 'nowrap',
          }}>查看详情</button>
          <button style={{ ...iconBtnB(), color: '#991b1b' }}>✕</button>
        </div>
      </div>
    }>
      <SourcesBodyDemo />
    </ShellB>
  );
}

// 2d. Top banner (success - more subtle, sticky)
function DemoBannerSuccess(props) {
  return (
    <ShellB {...props} page="sources" topbar={
      <div>
        <TopbarB page="sources"/>
        <div style={{
          padding: '8px 32px', background: '#f0fdf4', borderBottom: `1px solid #bbf7d0`,
          display: 'flex', alignItems: 'center', gap: 10,
        }}>
          <span style={{ color: '#16a34a', fontSize: 14, fontWeight: 700 }}>✓</span>
          <span style={{ fontSize: 12, color: '#166534' }}>
            <strong>已保存并热重载</strong>
            <span style={{ marginLeft: 12, color: '#15803d' }}>142ms · 14 节点 · 6 分组 · 5 路由</span>
          </span>
          <a style={{ marginLeft: 'auto', fontSize: 12, color: '#16a34a', cursor: 'pointer' }}>查看变更</a>
          <button style={{ ...iconBtnB(), color: '#166534' }}>✕</button>
        </div>
      </div>
    }>
      <SourcesBodyDemo />
    </ShellB>
  );
}

// =============================================================
// 3. Loading 状态
// =============================================================

function Spinner({ size = 14, color }) {
  return (
    <span style={{
      display: 'inline-block', width: size, height: size,
      border: `2px solid ${color || 'rgba(255,255,255,0.3)'}`,
      borderTopColor: color || '#fff', borderRadius: '50%',
      animation: 'b-spin 0.8s linear infinite',
    }}/>
  );
}

// 3a. 按钮内 spinner
function DemoLoadingButton(props) {
  return (
    <ShellB {...props} page="sources" topbar={
      <TopbarB page="sources" actions={
        <>
          <span style={{ fontSize: 12, color: B.textDim, marginRight: 4 }}>config.yaml</span>
          <button style={{ ...btnB('secondary'), opacity: 0.6, cursor: 'not-allowed' }} disabled>
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
              <Spinner size={12} color={B.accent}/>
              校验中…
            </span>
          </button>
          <button style={{ ...btnB('primary'), opacity: 0.6, cursor: 'not-allowed' }} disabled>保存并热重载</button>
        </>
      }/>
    }>
      <SourcesBodyDemo />
    </ShellB>
  );
}

// 3b. 顶部进度条
function DemoLoadingTopBar(props) {
  return (
    <ShellB {...props} page="sources" topbar={
      <div style={{ position: 'relative' }}>
        {/* progress bar */}
        <div style={{
          position: 'absolute', top: 0, left: 0, right: 0, height: 3,
          background: B.panelAlt, overflow: 'hidden', zIndex: 2,
        }}>
          <div style={{
            position: 'absolute', top: 0, height: '100%', width: '40%',
            background: B.accent,
            animation: 'b-indeterminate 1.4s ease-in-out infinite',
            boxShadow: `0 0 8px ${B.accent}`,
          }}/>
        </div>
        <TopbarB page="sources" actions={
          <>
            <span style={{ fontSize: 12, color: B.accent, marginRight: 8, fontWeight: 500 }}>正在拉取上游订阅 (2/3)…</span>
            <button style={{ ...btnB('secondary'), opacity: 0.6 }} disabled>校验</button>
            <button style={{ ...btnB('primary'), opacity: 0.6 }} disabled>保存并热重载</button>
          </>
        }/>
      </div>
    }>
      <SourcesBodyDemo />
    </ShellB>
  );
}

// 3c. 全屏遮罩
function DemoLoadingOverlay(props) {
  return (
    <ShellB {...props} page="sources">
      <div style={{ height: '100%', position: 'relative', overflow: 'hidden' }}>
        <SourcesBodyDemo />
        <div style={{
          position: 'absolute', inset: 0, background: 'rgba(255,255,255,0.7)',
          backdropFilter: 'blur(2px)',
          display: 'flex', alignItems: 'center', justifyContent: 'center', flexDirection: 'column', gap: 14,
        }}>
          <div style={{
            background: B.panel, border: `1px solid ${B.border}`, borderRadius: 12,
            padding: '24px 32px', boxShadow: '0 12px 32px rgba(15,23,42,0.15)',
            display: 'flex', alignItems: 'center', gap: 14,
          }}>
            <Spinner size={20} color={B.accent}/>
            <div>
              <div style={{ fontSize: 14, fontWeight: 600 }}>正在保存并热重载</div>
              <div style={{ fontSize: 12, color: B.textMuted, marginTop: 2 }}>请勿关闭页面</div>
            </div>
          </div>
        </div>
      </div>
    </ShellB>
  );
}

// =============================================================
// 4. 删除确认
// =============================================================

// 4a. 居中确认弹窗
function DemoConfirmModal(props) {
  return (
    <ShellB {...props} page="sources">
      <div style={{ height: '100%', position: 'relative', overflow: 'hidden' }}>
        <SourcesBodyDemo deletingIdx={1}/>
        <div style={{
          position: 'absolute', inset: 0, background: 'rgba(15,23,42,0.45)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
          <div style={{
            width: 420, background: B.panel, borderRadius: 14,
            boxShadow: '0 24px 48px rgba(15,23,42,0.25)', padding: '24px 28px',
          }}>
            <div style={{ display: 'flex', alignItems: 'flex-start', gap: 14, marginBottom: 14 }}>
              <div style={{
                width: 36, height: 36, borderRadius: 18, background: '#fee2e2',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                color: '#dc2626', fontWeight: 700, fontSize: 18, flex: '0 0 auto',
              }}>!</div>
              <div>
                <h3 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>删除订阅？</h3>
                <div style={{ fontSize: 13, color: B.textMuted, marginTop: 6, lineHeight: 1.5 }}>
                  即将删除「<strong style={{ color: B.text }}>备用订阅</strong>」（4 个节点）。该操作不可撤销，但你可以稍后重新添加。
                </div>
              </div>
            </div>
            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 18 }}>
              <button style={btnB('secondary')}>取消</button>
              <button style={{
                ...btnB('primary'),
                background: '#dc2626', borderColor: '#dc2626',
              }}>确认删除</button>
            </div>
          </div>
        </div>
      </div>
    </ShellB>
  );
}

// 4b. 行内确认
function DemoConfirmInline(props) {
  return (
    <ShellB {...props} page="sources">
      <div style={{ padding: '24px 32px' }}>
        <div style={{ display: 'flex', alignItems: 'center', marginBottom: 16 }}>
          <h2 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>SS 订阅</h2>
          <span style={{
            fontSize: 11, padding: '2px 8px', borderRadius: 999, marginLeft: 10,
            background: B.panelAlt, color: B.textMuted, fontWeight: 600, fontFamily: B.mono,
          }}>3</span>
          <button style={{ ...btnB('primary'), marginLeft: 'auto' }}>+ 添加订阅</button>
        </div>
        <div style={{
          background: B.panel, border: `1px solid ${B.border}`, borderRadius: 12, overflow: 'hidden',
        }}>
          <div style={{ padding: '14px 18px', borderBottom: `1px solid ${B.border}`, display: 'flex', alignItems: 'center', gap: 14 }}>
            <span style={{ width: 8, height: 8, borderRadius: 4, background: '#22c55e' }}/>
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: 14, fontWeight: 500 }}>主订阅</div>
              <code className="b-mono" style={{ fontSize: 11, color: B.textDim }}>https://nodes.example.org/sub/user_42/clash</code>
            </div>
            <span style={{ fontSize: 11, padding: '2px 8px', borderRadius: 4, background: B.panelAlt, color: B.textMuted, fontFamily: B.mono }}>12 节点</span>
            <button style={iconBtnB()}>✎</button>
            <button style={{ ...iconBtnB(), color: '#ef4444' }}>✕</button>
          </div>
          {/* row in confirming state */}
          <div style={{
            padding: '14px 18px', borderBottom: `1px solid ${B.border}`,
            background: '#fef2f2', borderLeft: '3px solid #dc2626',
            display: 'flex', alignItems: 'center', gap: 14,
          }}>
            <span style={{ width: 8, height: 8, borderRadius: 4, background: '#ef4444' }}/>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: 14, fontWeight: 500, color: '#991b1b', textDecoration: 'line-through' }}>备用订阅</div>
              <div style={{ fontSize: 12, color: '#7f1d1d', marginTop: 2 }}>确认删除？该操作不可撤销。</div>
            </div>
            <button style={{
              padding: '6px 12px', borderRadius: 6, border: 'none',
              background: '#dc2626', color: '#fff', fontSize: 12, fontWeight: 500, cursor: 'pointer',
              whiteSpace: 'nowrap',
            }}>确认删除</button>
            <button style={{
              padding: '6px 12px', borderRadius: 6, border: `1px solid #fca5a5`,
              background: '#fff', color: '#991b1b', fontSize: 12, cursor: 'pointer',
              whiteSpace: 'nowrap',
            }}>取消</button>
          </div>
          <div style={{ padding: '14px 18px', display: 'flex', alignItems: 'center', gap: 14 }}>
            <span style={{ width: 8, height: 8, borderRadius: 4, background: '#22c55e' }}/>
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: 14, fontWeight: 500 }}>次要源</div>
              <code className="b-mono" style={{ fontSize: 11, color: B.textDim }}>https://provider.example.net/clash/UserABC</code>
            </div>
            <span style={{ fontSize: 11, padding: '2px 8px', borderRadius: 4, background: B.panelAlt, color: B.textMuted, fontFamily: B.mono }}>2 节点</span>
            <button style={iconBtnB()}>✎</button>
            <button style={{ ...iconBtnB(), color: '#ef4444' }}>✕</button>
          </div>
        </div>
      </div>
    </ShellB>
  );
}

// 4c. 输入名称确认 (Stripe-style)
function DemoConfirmType(props) {
  return (
    <ShellB {...props} page="sources">
      <div style={{ height: '100%', position: 'relative', overflow: 'hidden' }}>
        <SourcesBodyDemo deletingIdx={1}/>
        <div style={{
          position: 'absolute', inset: 0, background: 'rgba(15,23,42,0.45)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
          <div style={{
            width: 460, background: B.panel, borderRadius: 14,
            boxShadow: '0 24px 48px rgba(15,23,42,0.25)', padding: '24px 28px',
          }}>
            <h3 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>删除订阅</h3>
            <div style={{ fontSize: 13, color: B.textMuted, marginTop: 8, lineHeight: 1.6 }}>
              即将永久删除「<strong style={{ color: B.text }}>备用订阅</strong>」及其拉取到的 4 个节点。引用该订阅的分组会重新计算。
            </div>
            <div style={{
              padding: '10px 14px', background: '#fef3c7', borderRadius: 8,
              fontSize: 12, color: '#78350f', margin: '14px 0', lineHeight: 1.5,
            }}>
              ⚠ 「🇯🇵 日本」「🇸🇬 新加坡」分组中的 3 个节点会消失
            </div>
            <FieldB label="请输入订阅名称以确认" hint="区分大小写">
              <input className="b-mono" placeholder="备用订阅" style={inputB()} />
            </FieldB>
            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 18 }}>
              <button style={btnB('secondary')}>取消</button>
              <button style={{
                ...btnB('primary'),
                background: '#94a3b8', borderColor: '#94a3b8', cursor: 'not-allowed',
              }} disabled>永久删除</button>
            </div>
          </div>
        </div>
      </div>
    </ShellB>
  );
}

// =============================================================
// 5. A8 跳转修复
// =============================================================

// 5a. 跳转 + 字段高亮
function DemoJumpHighlight(props) {
  return (
    <ShellB {...props} page="groups">
      <div style={{ padding: '24px 32px' }}>
        <div style={{
          padding: '10px 14px', background: '#fef3c7', borderRadius: 8,
          fontSize: 12, color: '#78350f', marginBottom: 16,
          display: 'flex', alignItems: 'center', gap: 10,
        }}>
          <span>来自校验：</span>
          <code className="b-mono" style={{ fontSize: 11 }}>groups[2].regex</code>
          <span style={{ marginLeft: 'auto' }}>← 上一个 · 下一个 →</span>
        </div>

        <div style={{ display: 'flex', gap: 10, marginBottom: 24, flexWrap: 'wrap' }}>
          {GROUPS.slice(0, 4).map((g, i) => (
            <div key={g.id} style={{
              padding: '10px 14px', borderRadius: 10,
              background: i === 2 ? '#fff7ed' : B.panel,
              color: B.text,
              border: `1px solid ${i === 2 ? '#f97316' : B.border}`,
              display: 'flex', alignItems: 'center', gap: 10,
              boxShadow: i === 2 ? '0 0 0 3px rgba(249,115,22,0.15)' : 'none',
              whiteSpace: 'nowrap', flex: '0 0 auto',
            }}>
              <span style={{ fontWeight: 500, fontSize: 13 }}>{g.name}</span>
              {i === 2 && <span style={{ fontSize: 11, color: '#9a3412', fontWeight: 600 }}>● 待修复</span>}
            </div>
          ))}
        </div>

        <div style={{
          background: B.panel, border: `1px solid ${B.border}`, borderRadius: 14,
          padding: '24px 28px',
        }}>
          <h2 style={{ margin: '0 0 18px', fontSize: 18, fontWeight: 600 }}>编辑分组</h2>
          <FieldB label="分组名称">
            <input defaultValue="🇨🇳 中国大陆" style={inputB()} />
          </FieldB>
          {/* highlighted field */}
          <div style={{ marginBottom: 18 }}>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 6 }}>
              <span style={{ fontSize: 13, fontWeight: 500, color: '#dc2626' }}>匹配正则</span>
              <span style={{ fontSize: 11, color: '#dc2626', fontFamily: B.mono }}>· 来自校验</span>
            </div>
            <input
              className="b-mono"
              defaultValue="(中国|China|CN|大陆[^)]"
              style={{
                ...inputB(),
                borderColor: '#ef4444', borderWidth: 2,
                color: '#991b1b',
                boxShadow: '0 0 0 4px rgba(239,68,68,0.1)',
                animation: 'b-pulse 2s ease-in-out infinite',
              }}
              autoFocus
            />
            <div style={{
              marginTop: 6, padding: '8px 12px', borderRadius: 6,
              background: '#fee2e2', fontSize: 12, color: '#991b1b',
            }}>
              ⚠ Unterminated character class at position 12
            </div>
          </div>
        </div>
      </div>
    </ShellB>
  );
}

// 5b. A8 旁边 Drawer 直接修
function DemoJumpDrawer(props) {
  return (
    <ShellB {...props} page="validate">
      <div style={{ height: '100%', position: 'relative', overflow: 'hidden' }}>
        <div style={{ height: '100%', overflow: 'auto', padding: '24px 32px', paddingRight: 480 }}>
          <div style={{
            background: B.panel, border: `1px solid ${B.border}`, borderRadius: 12, overflow: 'hidden',
          }}>
            {VALIDATION_ERRORS.slice(0, 3).map((e, i) => (
              <div key={e.id} style={{
                padding: '14px 18px', borderBottom: i < 2 ? `1px solid ${B.border}` : 'none',
                display: 'flex', gap: 14, alignItems: 'flex-start',
                background: i === 1 ? B.accent + '10' : B.panel,
                borderLeft: i === 1 ? `3px solid ${B.accent}` : '3px solid transparent',
              }}>
                <span style={{
                  width: 8, height: 8, borderRadius: 4, background: '#dc2626', marginTop: 7,
                }}/>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: 14, fontWeight: 600 }}>{e.message}</div>
                  <code className="b-mono" style={{
                    fontSize: 11, color: B.textMuted, display: 'inline-block', marginTop: 4,
                    padding: '2px 8px', borderRadius: 4, background: B.panelAlt,
                  }}>{e.field}</code>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Inline edit drawer */}
        <div style={{
          position: 'absolute', top: 0, right: 0, bottom: 0, width: 460,
          background: B.panel, borderLeft: `1px solid ${B.border}`,
          boxShadow: '-12px 0 24px rgba(15,23,42,0.08)',
          display: 'flex', flexDirection: 'column',
        }}>
          <div style={{ padding: '20px 24px', borderBottom: `1px solid ${B.border}`, display: 'flex', alignItems: 'center' }}>
            <div>
              <div style={{ fontSize: 11, color: B.textDim, fontFamily: B.mono, marginBottom: 4 }}>groups[2].regex</div>
              <h3 style={{ margin: 0, fontSize: 15, fontWeight: 600 }}>正则表达式语法错误</h3>
            </div>
            <button style={{ ...iconBtnB(), marginLeft: 'auto' }}>✕</button>
          </div>
          <div style={{ flex: 1, padding: '20px 24px', overflow: 'auto' }}>
            <div style={{
              padding: '10px 14px', background: '#fee2e2', borderRadius: 8,
              fontSize: 12, color: '#991b1b', marginBottom: 18, fontFamily: B.mono,
            }}>
              Unterminated character class at position 12
            </div>
            <FieldB label="分组" hint="🇨🇳 中国大陆">
              <input value="🇨🇳 中国大陆" style={{ ...inputB(), background: B.panelAlt }} readOnly />
            </FieldB>
            <FieldB label="正则" hint="实时校验">
              <input
                className="b-mono"
                defaultValue="(中国|China|CN|大陆[^)]"
                style={{ ...inputB(), borderColor: '#ef4444', color: '#991b1b' }}
              />
            </FieldB>
            <div style={{ fontSize: 12, color: B.textMuted, marginTop: 4 }}>
              建议：闭合字符类，如 <code className="b-mono" style={{ background: B.panelAlt, padding: '1px 6px', borderRadius: 4 }}>[^)]</code> → <code className="b-mono" style={{ background: B.panelAlt, padding: '1px 6px', borderRadius: 4 }}>[^\)]</code>
            </div>
          </div>
          <div style={{ padding: '14px 24px', borderTop: `1px solid ${B.border}`, display: 'flex', gap: 8, justifyContent: 'space-between' }}>
            <button style={btnB('secondary')}>跳过</button>
            <div style={{ display: 'flex', gap: 8 }}>
              <button style={btnB('secondary')}>下一个 →</button>
              <button style={btnB('primary')}>修复并继续</button>
            </div>
          </div>
        </div>
      </div>
    </ShellB>
  );
}

Object.assign(window, {
  DemoToastSuccess, DemoToastError, DemoBannerError, DemoBannerSuccess,
  DemoLoadingButton, DemoLoadingTopBar, DemoLoadingOverlay,
  DemoConfirmModal, DemoConfirmInline, DemoConfirmType,
  DemoJumpHighlight, DemoJumpDrawer,
  Spinner,
});
