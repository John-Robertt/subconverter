// B-3: Runtime preview screens — node preview, group preview, validation, health.

// ---- A8 配置校验 ----
function ValidateB(props) {
  const errors = VALIDATION_ERRORS.filter(e => e.severity === 'error');
  const warnings = VALIDATION_ERRORS.filter(e => e.severity === 'warning');
  const infos = VALIDATION_ERRORS.filter(e => e.severity === 'info');
  const ls = props.loadingState;

  return (
    <ShellB {...props} page="validate" topbar={
      <TopbarB page="validate" actions={
        <>
          <span style={{ fontSize: 12, color: B.textDim, marginRight: 8 }}>
            {ls === 'validating' ? '校验中…' : `最后校验：刚刚 · ${VALIDATION_ERRORS.length} 个问题`}
          </span>
          {ls === 'validating' ? (
            <button style={{ ...btnB('secondary'), opacity: 0.7, cursor: 'not-allowed' }} disabled>
              <Spinner size={12} color={B.accent}/>
              <span style={{ marginLeft: 6 }}>校验中…</span>
            </button>
          ) : (
            <button style={btnB('secondary')}>重新校验</button>
          )}
          <button style={btnB('primary')} disabled>保存（请先修复 {errors.length} 个错误）</button>
        </>
      }/>
    }>
      <div style={{ height: '100%', overflow: 'auto', padding: '28px 32px' }}>
        <div style={{
          display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 14, marginBottom: 22,
        }}>
          <SummaryStat label="错误" count={errors.length} tone="danger" desc="必须修复才能保存" />
          <SummaryStat label="警告" count={warnings.length} tone="warning" desc="建议修复但不阻塞" />
          <SummaryStat label="提示" count={infos.length} tone="info" desc="可选优化建议" />
        </div>

        <ErrorList title="错误" tone="danger" items={errors} />
        <ErrorList title="警告" tone="warning" items={warnings} />
        <ErrorList title="提示" tone="info" items={infos} />
      </div>
    </ShellB>
  );
}

function SummaryStat({ label, count, tone, desc }) {
  const tones = {
    danger: { fg: '#dc2626', bg: '#fee2e2', border: '#fecaca' },
    warning: { fg: '#d97706', bg: '#fef3c7', border: '#fde68a' },
    info: { fg: '#0284c7', bg: '#e0f2fe', border: '#bae6fd' },
  }[tone];
  return (
    <div style={{
      padding: '18px 20px', borderRadius: 12,
      background: tones.bg, border: `1px solid ${tones.border}`,
    }}>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
        <span style={{ fontSize: 28, fontWeight: 700, color: tones.fg, letterSpacing: '-0.02em' }}>{count}</span>
        <span style={{ fontSize: 13, fontWeight: 600, color: tones.fg }}>{label}</span>
      </div>
      <div style={{ fontSize: 12, color: tones.fg, opacity: 0.8, marginTop: 2 }}>{desc}</div>
    </div>
  );
}

function ErrorList({ title, tone, items }) {
  if (items.length === 0) return null;
  const dotColor = { danger: '#dc2626', warning: '#d97706', info: '#0284c7' }[tone];
  const PAGE_LABEL = {
    sources: '订阅来源', groups: '节点分组', routing: '路由策略',
    rulesets: '规则集', other: '其他配置',
  };
  return (
    <div style={{ marginBottom: 22 }}>
      <div style={{
        fontSize: 12, fontWeight: 600, color: B.textDim, textTransform: 'uppercase',
        letterSpacing: '0.06em', padding: '0 4px 10px',
      }}>{title} · {items.length}</div>
      <div style={{
        background: B.panel, border: `1px solid ${B.border}`, borderRadius: 12, overflow: 'hidden',
      }}>
        {items.map((e, i) => (
          <div key={e.id} style={{
            padding: '14px 18px', borderBottom: i < items.length - 1 ? `1px solid ${B.border}` : 'none',
            display: 'flex', gap: 14, alignItems: 'flex-start',
          }}>
            <span style={{
              width: 8, height: 8, borderRadius: 4, background: dotColor, marginTop: 7, flex: '0 0 8px',
            }}/>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                <span style={{ fontSize: 14, fontWeight: 600, color: B.text }}>{e.message}</span>
                <code className="b-mono" style={{
                  fontSize: 11, padding: '2px 8px', borderRadius: 4,
                  background: B.panelAlt, color: B.textMuted,
                }}>{e.field}</code>
              </div>
              <div style={{ fontSize: 13, color: B.textMuted, marginTop: 4, lineHeight: 1.5 }}>
                {e.detail}
              </div>
            </div>
            <button style={{
              padding: '5px 12px', borderRadius: 6, border: `1px solid ${B.border}`,
              background: B.panel, color: B.textMuted, fontSize: 12, cursor: 'pointer',
              whiteSpace: 'nowrap',
            }}>跳转 → {PAGE_LABEL[e.page] || e.page}</button>
          </div>
        ))}
      </div>
    </div>
  );
}

// ---- B1 节点预览 ----
function NodePreviewB(props) {
  const cats = [
    { id: 'all', label: '全部', count: NODES.length },
    { id: 'sub', label: '订阅', count: NODES.filter(n => n.kind === 'sub').length },
    { id: 'snell', label: 'Snell', count: NODES.filter(n => n.kind === 'snell').length, only: 'Surge' },
    { id: 'vless', label: 'VLESS', count: NODES.filter(n => n.kind === 'vless').length, only: 'Clash' },
    { id: 'custom', label: '自定义', count: NODES.filter(n => n.kind === 'custom').length },
  ];

  return (
    <ShellB {...props} page="preview" topbar={
      <TopbarB page="preview" actions={
        <>
          <span style={{ fontSize: 12, color: B.textDim, marginRight: 8 }}>最后拉取：12 秒前</span>
          <button style={btnB('secondary')}>↻ 重新拉取</button>
        </>
      }/>
    }>
      <div style={{ height: '100%', overflow: 'auto', padding: '24px 32px' }}>
        <div style={{ display: 'flex', gap: 8, marginBottom: 18, flexWrap: 'wrap' }}>
          {cats.map((c, i) => (
            <button key={c.id} style={{
              padding: '8px 14px', borderRadius: 999,
              border: `1px solid ${i === 0 ? B.accent : B.border}`,
              background: i === 0 ? B.accent : B.panel,
              color: i === 0 ? '#fff' : B.text,
              fontSize: 13, fontWeight: 500, cursor: 'pointer',
              display: 'inline-flex', alignItems: 'center', gap: 8,
            }}>
              {c.label}
              <span style={{
                fontSize: 11, padding: '1px 7px', borderRadius: 999,
                background: i === 0 ? 'rgba(255,255,255,0.25)' : B.panelAlt,
                color: i === 0 ? '#fff' : B.textMuted,
                fontFamily: B.mono, fontVariantNumeric: 'tabular-nums',
              }}>{c.count}</span>
              {c.only && <span style={{
                fontSize: 10, padding: '1px 5px', borderRadius: 3,
                background: c.only === 'Surge' ? '#fed7aa' : '#bfdbfe',
                color: c.only === 'Surge' ? '#9a3412' : '#1e40af',
                fontWeight: 600,
              }}>{c.only} only</span>}
            </button>
          ))}
        </div>

        <div style={{
          background: B.panel, border: `1px solid ${B.border}`, borderRadius: 12, overflow: 'hidden',
        }}>
          <div style={{
            display: 'grid',
            gridTemplateColumns: '32px 1.5fr 100px 1fr 80px 90px 90px',
            padding: '12px 18px', background: B.panelAlt,
            fontSize: 11, fontWeight: 600, color: B.textDim,
            textTransform: 'uppercase', letterSpacing: '0.05em',
            borderBottom: `1px solid ${B.border}`,
          }}>
            <span></span>
            <span>名称</span>
            <span>类型</span>
            <span>服务器</span>
            <span>端口</span>
            <span>来源</span>
            <span>标签</span>
          </div>
          {NODES.map((n, i) => (
            <div key={i} style={{
              display: 'grid',
              gridTemplateColumns: '32px 1.5fr 100px 1fr 80px 90px 90px',
              padding: '11px 18px',
              borderBottom: i < NODES.length - 1 ? `1px solid ${B.border}` : 'none',
              fontSize: 13, alignItems: 'center',
            }}>
              <span style={{
                width: 6, height: 6, borderRadius: 3,
                background: n.alive === false ? '#ef4444' : '#22c55e',
              }}/>
              <span style={{ fontWeight: 500, color: B.text }}>{n.name}</span>
              <code className="b-mono" style={{ fontSize: 12, color: B.textMuted }}>{n.type}</code>
              <code className="b-mono" style={{ fontSize: 12, color: B.textMuted, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{n.server}</code>
              <code className="b-mono" style={{ fontSize: 12, color: B.textMuted, fontVariantNumeric: 'tabular-nums' }}>{n.port}</code>
              <span style={{
                fontSize: 11, padding: '2px 8px', borderRadius: 4,
                background: B.panelAlt, color: B.textMuted, fontFamily: B.mono,
                width: 'fit-content',
              }}>{n.kind}</span>
              <span>
                {n.kind === 'snell' && <span style={{
                  fontSize: 10, padding: '2px 6px', borderRadius: 3,
                  background: '#fed7aa', color: '#9a3412', fontWeight: 600,
                }}>Surge</span>}
                {n.kind === 'vless' && <span style={{
                  fontSize: 10, padding: '2px 6px', borderRadius: 3,
                  background: '#bfdbfe', color: '#1e40af', fontWeight: 600,
                }}>Clash</span>}
              </span>
            </div>
          ))}
        </div>
      </div>
    </ShellB>
  );
}

// ---- B3 分组预览 ----
function GroupPreviewB(props) {
  return (
    <ShellB {...props} page="grouppreview">
      <div style={{ height: '100%', overflow: 'auto', padding: '28px 32px' }}>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 24 }}>
          {GROUPS.slice(0, 6).map(g => {
            let re; try { re = new RegExp(g.regex); } catch {}
            const matched = re ? NODES.filter(n => re.test(n.name)) : [];
            return (
              <div key={g.id} style={{
                background: B.panel, border: `1px solid ${B.border}`, borderRadius: 12,
                padding: '16px 20px', boxShadow: '0 1px 2px rgba(0,0,0,0.03)',
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 10 }}>
                  <h3 style={{ margin: 0, fontSize: 15, fontWeight: 600 }}>{g.name}</h3>
                  <span style={{
                    fontSize: 11, padding: '1px 7px', borderRadius: 999, fontFamily: B.mono,
                    background: B.panelAlt, color: B.textMuted, fontWeight: 600,
                  }}>{matched.length}</span>
                  <span style={{
                    marginLeft: 'auto', fontSize: 11, padding: '2px 8px', borderRadius: 4,
                    background: g.strategy === 'select' ? '#dbeafe' : '#dcfce7',
                    color: g.strategy === 'select' ? '#1e40af' : '#166534',
                    fontFamily: B.mono, fontWeight: 600,
                  }}>{g.strategy}</span>
                </div>
                <code className="b-mono" style={{
                  display: 'block', padding: '6px 10px', background: B.panelAlt, borderRadius: 6,
                  fontSize: 11, color: B.textMuted, marginBottom: 10,
                  overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                }}>{g.regex}</code>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 4, maxHeight: 180, overflow: 'auto' }}>
                  {matched.slice(0, 8).map((n, i) => (
                    <div key={i} style={{
                      fontSize: 12, color: B.textMuted, padding: '3px 0',
                      overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                    }}>{n.name}</div>
                  ))}
                  {matched.length > 8 && (
                    <div style={{ fontSize: 11, color: B.textDim, padding: '2px 0', fontStyle: 'italic' }}>
                      … 还有 {matched.length - 8} 个
                    </div>
                  )}
                  {matched.length === 0 && (
                    <div style={{ fontSize: 12, color: '#dc2626', padding: '6px 0' }}>
                      ⚠ 未匹配到任何节点
                    </div>
                  )}
                </div>
              </div>
            );
          })}
        </div>

        <div style={{ fontSize: 12, fontWeight: 600, color: B.textDim, textTransform: 'uppercase', letterSpacing: '0.06em', padding: '0 4px 10px' }}>
          服务组展开 / Expansion
        </div>
        <div style={{
          background: B.panel, border: `1px solid ${B.border}`, borderRadius: 12,
          padding: '18px 22px',
        }}>
          {ROUTING.slice(0, 3).map((r, i) => (
            <div key={r.id} style={{
              padding: '14px 0',
              borderBottom: i < 2 ? `1px solid ${B.border}` : 'none',
              display: 'flex', gap: 16, alignItems: 'flex-start',
            }}>
              <div style={{ flex: '0 0 200px' }}>
                <div style={{ fontSize: 14, fontWeight: 600 }}>{r.name}</div>
                <div style={{ fontSize: 11, color: B.textDim, marginTop: 2, fontFamily: B.mono }}>
                  {r.members.length} 成员 → {(r.members.length === 1 && r.members[0] === '@all') ? NODES.length : Math.floor(NODES.length * 0.7)} 节点
                </div>
              </div>
              <div style={{ flex: 1, display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                {(r.id === 'global' ? GROUPS.slice(0, 6) : GROUPS.slice(0, 3)).map(g => (
                  <span key={g.id} style={{
                    fontSize: 11, padding: '2px 8px', borderRadius: 4,
                    background: B.panelAlt, color: B.textMuted,
                  }}>{g.name}</span>
                ))}
                <span style={{
                  fontSize: 11, padding: '2px 8px', borderRadius: 4,
                  background: '#dcfce7', color: '#166534', fontFamily: B.mono, fontWeight: 600,
                }}>DIRECT</span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </ShellB>
  );
}

// ---- D 系统状态 ----
function HealthB(props) {
  return (
    <ShellB {...props} page="health">
      <div style={{ height: '100%', overflow: 'auto', padding: '28px 32px' }}>
        <div style={{
          display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 14, marginBottom: 22,
        }}>
          <HealthStat
            label="服务状态" value="运行中" tone="success"
            sub="PID 28471 · uptime 6d 14h"
          />
          <HealthStat
            label="版本" value="v0.9.4" tone="neutral"
            sub="git 7a3f2d1 · 2026-04-22"
          />
          <HealthStat
            label="配置" value="已加载" tone="success"
            sub="14 节点 · 6 分组 · 5 路由"
          />
          <HealthStat
            label="上次热重载" value="2 分钟前" tone="info"
            sub="142ms · 来自 webhook"
          />
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: '1.4fr 1fr', gap: 16 }}>
          <div style={{
            background: B.panel, border: `1px solid ${B.border}`, borderRadius: 12,
            padding: '18px 22px',
          }}>
            <div style={{ display: 'flex', alignItems: 'center', marginBottom: 14 }}>
              <h3 style={{ margin: 0, fontSize: 15, fontWeight: 600 }}>热重载历史</h3>
              <span style={{ marginLeft: 'auto', fontSize: 12, color: B.textDim }}>近 7 次 · </span>
              <a style={{ fontSize: 12, color: B.accent, textDecoration: 'none', cursor: 'pointer', marginLeft: 4 }}>查看全部</a>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {RELOAD_HISTORY.map((h, i) => (
                <div key={i} style={{
                  padding: '10px 12px', borderRadius: 8, background: B.panelAlt,
                  display: 'grid', gridTemplateColumns: '90px 60px 1fr 70px', gap: 12,
                  fontSize: 12, alignItems: 'center',
                }}>
                  <code className="b-mono" style={{ color: B.textMuted, fontVariantNumeric: 'tabular-nums' }}>{h.time}</code>
                  <span style={{
                    fontSize: 11, padding: '2px 6px', borderRadius: 4, fontWeight: 600,
                    background: h.status === 'success' ? '#dcfce7' : '#fee2e2',
                    color: h.status === 'success' ? '#166534' : '#991b1b',
                    width: 'fit-content', fontFamily: B.mono,
                  }}>{h.status === 'success' ? '✓ ok' : '✕ failed'}</span>
                  <span style={{ color: B.text, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{h.changes}</span>
                  <code className="b-mono" style={{
                    color: h.status === 'success' ? B.textMuted : '#dc2626',
                    fontVariantNumeric: 'tabular-nums', textAlign: 'right',
                  }}>{h.dur}</code>
                </div>
              ))}
            </div>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div style={{
              background: B.panel, border: `1px solid ${B.border}`, borderRadius: 12,
              padding: '18px 22px',
            }}>
              <h3 style={{ margin: '0 0 14px', fontSize: 15, fontWeight: 600 }}>运行环境</h3>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 10, fontSize: 13 }}>
                {[
                  ['配置文件', '/etc/subconverter/config.yaml'],
                  ['监听地址', '0.0.0.0:25500'],
                  ['工作目录', '/var/lib/subconverter'],
                  ['Go runtime', 'go1.22.3 linux/amd64'],
                  ['内存占用', '38.4 MB'],
                  ['请求总数', '12,847 (过去 24h)'],
                ].map(([k, v]) => (
                  <div key={k} style={{ display: 'flex', alignItems: 'baseline', gap: 12 }}>
                    <span style={{ flex: '0 0 90px', color: B.textDim, fontSize: 12 }}>{k}</span>
                    <code className="b-mono" style={{ flex: 1, color: B.text, fontSize: 12, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{v}</code>
                  </div>
                ))}
              </div>
            </div>

            <div style={{
              background: B.panel, border: `1px solid ${B.border}`, borderRadius: 12,
              padding: '18px 22px',
            }}>
              <h3 style={{ margin: '0 0 14px', fontSize: 15, fontWeight: 600 }}>探针 / Endpoints</h3>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                <ProbeRow method="GET" path="/healthz" status="200" />
                <ProbeRow method="GET" path="/generate?format=clash" status="200" />
                <ProbeRow method="GET" path="/generate?format=surge" status="200" />
                <ProbeRow method="POST" path="/reload" status="—" />
              </div>
            </div>
          </div>
        </div>
      </div>
    </ShellB>
  );
}

function HealthStat({ label, value, sub, tone }) {
  const tones = {
    success: '#16a34a', neutral: B.text, info: B.accent, danger: '#dc2626',
  };
  return (
    <div style={{
      background: B.panel, border: `1px solid ${B.border}`, borderRadius: 12,
      padding: '16px 18px', boxShadow: '0 1px 2px rgba(0,0,0,0.03)',
    }}>
      <div style={{ fontSize: 11, color: B.textDim, fontWeight: 500, textTransform: 'uppercase', letterSpacing: '0.05em' }}>{label}</div>
      <div style={{ fontSize: 22, fontWeight: 700, color: tones[tone], letterSpacing: '-0.02em', margin: '6px 0 4px', display: 'flex', alignItems: 'center', gap: 8 }}>
        {tone === 'success' && <span style={{ width: 8, height: 8, borderRadius: 4, background: '#22c55e', display: 'inline-block', boxShadow: '0 0 0 4px rgba(34,197,94,0.2)' }}/>}
        {value}
      </div>
      <div style={{ fontSize: 11, color: B.textMuted, fontFamily: B.mono, fontVariantNumeric: 'tabular-nums' }}>{sub}</div>
    </div>
  );
}

function ProbeRow({ method, path, status }) {
  return (
    <div style={{
      padding: '8px 10px', borderRadius: 6, background: B.panelAlt,
      display: 'flex', alignItems: 'center', gap: 10,
    }}>
      <span style={{
        fontSize: 10, padding: '2px 6px', borderRadius: 3, fontFamily: B.mono, fontWeight: 700,
        background: method === 'GET' ? '#dcfce7' : '#fef3c7',
        color: method === 'GET' ? '#166534' : '#854d0e',
      }}>{method}</span>
      <code className="b-mono" style={{ flex: 1, fontSize: 12, color: B.text, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{path}</code>
      <code className="b-mono" style={{
        fontSize: 11, padding: '1px 6px', borderRadius: 3,
        background: status === '200' ? '#dcfce7' : B.panel,
        color: status === '200' ? '#166534' : B.textMuted,
        fontWeight: 600,
      }}>{status}</code>
    </div>
  );
}

Object.assign(window, { ValidateB, NodePreviewB, GroupPreviewB, HealthB });
