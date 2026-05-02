// B-2: Filters, Routing, Rulesets, Rules, Other config screens.

// ---- A2 过滤器 ----
function FiltersB(props) {
  const [regex, setRegex] = React.useState(FILTERS.exclude);
  const [debounced, setDebounced] = React.useState(regex);
  React.useEffect(() => {
    const t = setTimeout(() => setDebounced(regex), 300);
    return () => clearTimeout(t);
  }, [regex]);
  let valid = true; let re;
  try { re = new RegExp(debounced); } catch { valid = false; }
  const total = NODES.length;
  const excluded = valid ? NODES.filter(n => re.test(n.name)) : [];
  const remaining = total - excluded.length;

  return (
    <ShellB {...props} page="filters">
      <div style={{ height: '100%', display: 'flex', overflow: 'hidden' }}>
        <div style={{ flex: 1, padding: '28px 32px', overflow: 'auto', minWidth: 0 }}>
          <div style={{
            background: B.panel, border: `1px solid ${B.border}`, borderRadius: 14,
            padding: '24px 28px', boxShadow: '0 1px 3px rgba(0,0,0,0.04)', marginBottom: 18,
          }}>
            <h2 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>排除规则</h2>
            <div style={{ fontSize: 13, color: B.textMuted, marginTop: 4, marginBottom: 18 }}>
              用正则匹配会被剔除的节点名，比如流量信息、官网、套餐到期等占位条目。
            </div>
            <FieldB label="exclude 正则" hint="300ms 后实时计算被剔除的节点">
              <input
                value={regex}
                onChange={e => setRegex(e.target.value)}
                className="b-mono"
                style={{ ...inputB(), borderColor: valid ? B.border : '#ef4444', color: valid ? B.text : '#ef4444' }}
              />
              {!valid && <div style={{ fontSize: 12, color: '#ef4444', marginTop: 6 }}>⚠ 正则语法错误</div>}
            </FieldB>

            <div style={{
              display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12, marginTop: 6,
            }}>
              <BigStatB label="原始节点" value={total} tone="neutral" />
              <BigStatB label="剔除" value={excluded.length} tone="danger" />
              <BigStatB label="保留" value={remaining} tone="success" />
            </div>
          </div>

          <div style={{ fontSize: 12, fontWeight: 600, color: B.textDim, textTransform: 'uppercase', letterSpacing: '0.06em', padding: '0 4px 10px' }}>常用模板</div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
            {[
              ['流量信息', '剩余|流量|套餐|到期|Expire'],
              ['官网链接', '官网|website|官方'],
              ['测试节点', '测试|Test|Demo'],
              ['IPv6', 'IPv6|v6'],
              ['高倍率', '×|x[2-9]|高倍|高级'],
            ].map(([name, p]) => (
              <button
                key={name}
                onClick={() => setRegex(prev => prev ? prev + '|' + p : p)}
                style={{
                  padding: '6px 12px', borderRadius: 999, border: `1px solid ${B.border}`,
                  background: B.panel, color: B.textMuted, fontSize: 12, cursor: 'pointer',
                }}
              >+ {name}</button>
            ))}
          </div>
        </div>

        <div style={{
          width: 380, flex: '0 0 380px', borderLeft: `1px solid ${B.border}`,
          background: B.panel, display: 'flex', flexDirection: 'column', overflow: 'hidden',
        }}>
          <div style={{ padding: '20px 22px 14px', borderBottom: `1px solid ${B.border}` }}>
            <div style={{ fontSize: 12, fontWeight: 600, color: B.textDim, textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: 6 }}>预览</div>
            <div style={{ fontSize: 13, color: B.textMuted }}>左侧 <span style={{ color: '#ef4444', fontWeight: 600 }}>● {excluded.length}</span> 个节点会被过滤</div>
          </div>
          <div style={{ flex: 1, overflow: 'auto', padding: '8px 0' }}>
            {NODES.map((n, i) => {
              const isExcluded = valid && re.test(n.name);
              return (
                <div key={i} style={{
                  padding: '8px 22px', display: 'flex', alignItems: 'center', gap: 10,
                  fontSize: 13, opacity: isExcluded ? 0.5 : 1,
                }}>
                  <span style={{
                    fontSize: 11, padding: '1px 6px', borderRadius: 4,
                    background: isExcluded ? '#fee2e2' : '#dcfce7',
                    color: isExcluded ? '#991b1b' : '#166534',
                    fontWeight: 600, fontFamily: B.mono,
                  }}>{isExcluded ? '剔除' : '保留'}</span>
                  <span style={{
                    flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                    textDecoration: isExcluded ? 'line-through' : 'none',
                    color: isExcluded ? B.textDim : B.text,
                  }}>{n.name}</span>
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </ShellB>
  );
}

function BigStatB({ label, value, tone }) {
  const tones = {
    neutral: { fg: B.text, bg: B.panelAlt },
    success: { fg: '#16a34a', bg: '#dcfce7' },
    danger: { fg: '#dc2626', bg: '#fee2e2' },
  }[tone] || { fg: B.text, bg: B.panelAlt };
  return (
    <div style={{ padding: '14px 16px', borderRadius: 10, background: tones.bg }}>
      <div style={{ fontSize: 11, color: B.textMuted, fontWeight: 500, marginBottom: 6 }}>{label}</div>
      <div style={{ fontSize: 24, fontWeight: 700, color: tones.fg, letterSpacing: '-0.02em' }}>{value}</div>
    </div>
  );
}

// ---- A4 路由策略 ----
function RoutingB(props) {
  const SPECIALS = [
    { id: '@all', label: '@all', desc: '所有节点', tone: 'gray' },
    { id: '@auto', label: '@auto', desc: '自动选择子组', tone: 'indigo' },
    { id: 'DIRECT', label: 'DIRECT', desc: '直连', tone: 'green' },
    { id: 'REJECT', label: 'REJECT', desc: '拒绝', tone: 'red' },
  ];

  return (
    <ShellB {...props} page="routing">
      <div style={{ height: '100%', display: 'flex', overflow: 'hidden' }}>
        <div style={{ flex: 1, padding: '28px 32px', overflow: 'auto', minWidth: 0 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            {ROUTING.map((r, idx) => {
              const active = idx === 0;
              return (
                <div key={r.id} style={{
                  background: B.panel, border: `1px solid ${active ? B.accent : B.border}`,
                  borderRadius: 14, padding: '18px 22px',
                  boxShadow: active ? '0 4px 12px rgba(99,102,241,0.15)' : '0 1px 2px rgba(0,0,0,0.03)',
                }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 14 }}>
                    <span style={{ color: B.textDim, cursor: 'grab', fontSize: 14 }}>⠿</span>
                    <span style={{ fontSize: 11, color: B.textDim, fontFamily: B.mono, fontVariantNumeric: 'tabular-nums' }}>{String(idx + 1).padStart(2, '0')}</span>
                    <h3 style={{ margin: 0, fontSize: 16, fontWeight: 600, whiteSpace: 'nowrap' }}>{r.name}</h3>
                    <span style={{
                      fontSize: 11, padding: '2px 8px', borderRadius: 999,
                      background: B.panelAlt, color: B.textMuted, fontWeight: 500,
                      whiteSpace: 'nowrap', flex: '0 0 auto',
                    }}>{r.members.length} 个成员</span>
                    <button style={{ ...iconBtnB(), marginLeft: 'auto' }}>✎</button>
                    <button style={{ ...iconBtnB(), color: '#ef4444' }}>✕</button>
                  </div>

                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, alignItems: 'center' }}>
                    {r.members.map((m, i) => (
                      <MemberChipB key={i} m={m} />
                    ))}
                    <button style={{
                      padding: '5px 12px', borderRadius: 8, border: `1.5px dashed ${B.border}`,
                      background: 'transparent', color: B.accent, fontSize: 12, fontWeight: 500, cursor: 'pointer',
                      whiteSpace: 'nowrap', flex: '0 0 auto',
                    }}>+ 添加成员</button>
                  </div>
                </div>
              );
            })}

            <div style={{
              padding: '14px 18px', border: `1.5px dashed ${B.border}`, borderRadius: 12,
              color: B.accent, fontSize: 13, fontWeight: 500, cursor: 'pointer',
              textAlign: 'center', background: 'transparent',
            }}>+ 新建服务组</div>
          </div>
        </div>

        <div style={{
          width: 320, flex: '0 0 320px', borderLeft: `1px solid ${B.border}`,
          background: B.panel, padding: '20px 22px', overflow: 'auto',
        }}>
          <div style={{ fontSize: 12, fontWeight: 600, color: B.textDim, textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: 12 }}>可选成员</div>
          <div style={{ fontSize: 12, color: B.textMuted, marginBottom: 8 }}>特殊关键字</div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: 18 }}>
            {SPECIALS.map(s => (
              <div key={s.id} style={{
                padding: '6px 10px', borderRadius: 8, border: `1px solid ${B.border}`,
                background: B.panel, fontSize: 12, cursor: 'grab',
                whiteSpace: 'nowrap', flex: '0 0 auto',
              }}>
                <div style={{ fontWeight: 600, fontFamily: B.mono }}>{s.label}</div>
                <div style={{ fontSize: 10, color: B.textDim, whiteSpace: 'nowrap' }}>{s.desc}</div>
              </div>
            ))}
          </div>
          <div style={{ fontSize: 12, color: B.textMuted, marginBottom: 8 }}>节点分组（{GROUPS.length}）</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4, marginBottom: 18 }}>
            {GROUPS.map(g => (
              <div key={g.id} style={{
                padding: '6px 10px', borderRadius: 6, fontSize: 12, cursor: 'grab',
                color: B.text, display: 'flex', alignItems: 'center', gap: 8,
              }}>
                <span style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{g.name}</span>
                <span style={{ marginLeft: 'auto', color: B.textDim, fontSize: 11, fontFamily: B.mono, whiteSpace: 'nowrap', flex: '0 0 auto' }}>{g.strategy}</span>
              </div>
            ))}
          </div>
          <div style={{
            padding: 12, borderRadius: 8, background: '#fef3c7', border: '1px solid #fcd34d',
            fontSize: 11, color: '#78350f', lineHeight: 1.6,
          }}>
            <strong>约束提示</strong>
            <div>· @all 和 @auto 不能同时出现</div>
            <div>· @auto 每组最多一个</div>
            <div>· REJECT 不会包含在 @auto 里</div>
          </div>
        </div>
      </div>
    </ShellB>
  );
}

function MemberChipB({ m }) {
  const isSpecial = m.startsWith('@') || m === 'DIRECT' || m === 'REJECT';
  let label = m, color = null;
  if (m === 'DIRECT') color = { bg: '#dcfce7', fg: '#166534' };
  else if (m === 'REJECT') color = { bg: '#fee2e2', fg: '#991b1b' };
  else if (m.startsWith('@')) color = { bg: '#e0e7ff', fg: '#3730a3' };
  else {
    const g = GROUPS.find(x => x.id === m);
    const r = ROUTING.find(x => x.id === m);
    label = g ? g.name : (r ? r.name : m);
  }
  return (
    <span style={{
      padding: '5px 10px', borderRadius: 8,
      background: color ? color.bg : '#f1f5f9',
      color: color ? color.fg : '#0f172a',
      fontSize: 12, fontWeight: isSpecial ? 600 : 500,
      fontFamily: isSpecial ? B.mono : 'inherit',
      display: 'inline-flex', alignItems: 'center', gap: 6,
      whiteSpace: 'nowrap', flex: '0 0 auto',
    }}>
      {label}
      <span style={{ opacity: 0.5, cursor: 'pointer' }}>×</span>
    </span>
  );
}

// ---- A5 规则集 ----
function RulesetsB(props) {
  return (
    <ShellB {...props} page="rulesets">
      <div style={{ height: '100%', overflow: 'auto', padding: '28px 32px' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          {ROUTING.map(r => {
            const urls = RULESETS[r.id] || [];
            return (
              <div key={r.id} style={{
                background: B.panel, border: `1px solid ${B.border}`, borderRadius: 14,
                padding: '18px 22px', boxShadow: '0 1px 2px rgba(0,0,0,0.03)',
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12 }}>
                  <h3 style={{ margin: 0, fontSize: 15, fontWeight: 600 }}>{r.name}</h3>
                  <span style={{
                    fontSize: 11, padding: '2px 8px', borderRadius: 999,
                    background: B.panelAlt, color: B.textMuted, fontWeight: 500, fontFamily: B.mono,
                  }}>{urls.length} 条 URL</span>
                  <span style={{ fontSize: 12, color: B.textDim }}>{r.members.length} 个路由成员</span>
                </div>

                {urls.length === 0 ? (
                  <div style={{
                    padding: '14px 16px', border: `1.5px dashed ${B.border}`, borderRadius: 10,
                    color: B.textDim, fontSize: 12, textAlign: 'center',
                  }}>
                    该服务组未挂载任何规则集
                  </div>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                    {urls.map((url, i) => (
                      <div key={i} style={{
                        padding: '10px 14px', borderRadius: 8, background: B.panelAlt,
                        display: 'flex', alignItems: 'center', gap: 10,
                      }}>
                        <span style={{ color: B.textDim, cursor: 'grab' }}>⠿</span>
                        <code className="b-mono" style={{
                          flex: 1, fontSize: 12, color: B.textMuted, overflow: 'hidden',
                          textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                        }}>{url}</code>
                        <span style={{ fontSize: 11, color: '#16a34a', fontFamily: B.mono }}>342 行</span>
                        <button style={iconBtnB()}>↻</button>
                        <button style={{ ...iconBtnB(), color: '#ef4444' }}>✕</button>
                      </div>
                    ))}
                  </div>
                )}

                <button style={{
                  marginTop: 10, padding: '8px 14px', borderRadius: 8, border: `1.5px dashed ${B.border}`,
                  background: 'transparent', color: B.accent, fontSize: 12, fontWeight: 500,
                  cursor: 'pointer', width: '100%',
                }}>+ 添加规则集 URL</button>
              </div>
            );
          })}
        </div>
      </div>
    </ShellB>
  );
}

// ---- A6 内联规则 ----
function RulesB(props) {
  const TYPE_COLORS = {
    'DOMAIN': '#06b6d4', 'DOMAIN-SUFFIX': '#0891b2', 'DOMAIN-KEYWORD': '#0e7490',
    'IP-CIDR': '#a855f7', 'GEOIP': '#d946ef', 'PROCESS-NAME': '#f59e0b',
    'MATCH': '#ef4444',
  };
  const TARGET_COLORS = {
    'DIRECT': '#16a34a', 'REJECT': '#dc2626',
  };

  return (
    <ShellB {...props} page="rules">
      <div style={{ height: '100%', overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
        <div style={{
          padding: '20px 32px 14px', display: 'flex', alignItems: 'center', gap: 16, flex: '0 0 auto',
        }}>
          <div style={{
            position: 'relative', flex: 1, maxWidth: 360,
          }}>
            <input
              placeholder="搜索规则…"
              style={{ ...inputB(), paddingLeft: 36, height: 36 }}
            />
            <span style={{
              position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)',
              color: B.textDim,
            }}>
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6"><circle cx="7" cy="7" r="4.5"/><path d="M10.5 10.5l3 3"/></svg>
            </span>
          </div>
          <span style={{ fontSize: 12, color: B.textMuted }}>共 {INLINE_RULES.length} 条 · 拖拽 ⠿ 调整顺序</span>
          <button style={{ ...btnB('secondary'), marginLeft: 'auto' }}>批量编辑</button>
          <button style={btnB('primary')}>+ 添加规则</button>
        </div>

        <div style={{ flex: 1, overflow: 'auto', padding: '6px 32px 28px', minHeight: 0 }}>
          <div style={{
            background: B.panel, border: `1px solid ${B.border}`, borderRadius: 12, overflow: 'hidden',
          }}>
            {INLINE_RULES.map((r, i) => (
              <div key={i} style={{
                padding: '10px 16px', borderBottom: i < INLINE_RULES.length - 1 ? `1px solid ${B.border}` : 'none',
                display: 'flex', alignItems: 'center', gap: 12,
              }}>
                <span style={{ color: B.textDim, cursor: 'grab' }}>⠿</span>
                <span style={{
                  fontSize: 11, color: B.textDim, fontFamily: B.mono,
                  fontVariantNumeric: 'tabular-nums', minWidth: 22,
                }}>{String(i + 1).padStart(2, '0')}</span>
                <span style={{
                  fontSize: 11, padding: '3px 8px', borderRadius: 6, fontFamily: B.mono, fontWeight: 600,
                  background: (TYPE_COLORS[r.type] || B.textMuted) + '1a',
                  color: TYPE_COLORS[r.type] || B.textMuted,
                  minWidth: 110, textAlign: 'center', boxSizing: 'border-box',
                }}>{r.type}</span>
                <code className="b-mono" style={{
                  flex: 1, fontSize: 13, color: B.text,
                  overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                }}>{r.match || <span style={{ color: B.textDim, fontStyle: 'italic' }}>(no match)</span>}</code>
                <span style={{ color: B.textDim, fontSize: 12 }}>→</span>
                <span style={{
                  fontSize: 12, padding: '3px 10px', borderRadius: 6, fontWeight: 500,
                  background: (TARGET_COLORS[r.target] || B.accent) + '1a',
                  color: TARGET_COLORS[r.target] || B.accent,
                  fontFamily: ['DIRECT', 'REJECT'].includes(r.target) ? B.mono : 'inherit',
                }}>{r.target}</span>
                {r.noResolve && <span style={{
                  fontSize: 10, padding: '2px 6px', borderRadius: 4, background: B.panelAlt,
                  color: B.textMuted, fontFamily: B.mono,
                }}>no-resolve</span>}
                <button style={iconBtnB()}>✎</button>
                <button style={{ ...iconBtnB(), color: '#ef4444' }}>✕</button>
              </div>
            ))}
          </div>
        </div>
      </div>
    </ShellB>
  );
}

// ---- A7 其他配置 ----
function OtherB(props) {
  return (
    <ShellB {...props} page="other">
      <div style={{ height: '100%', overflow: 'auto', padding: '28px 32px' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16, maxWidth: 720 }}>
          <CardSetting
            title="fallback 服务组"
            desc="所有规则都不匹配时使用的兜底。建议指向「全球代理」类的服务组。"
          >
            <select style={{ ...inputB(), width: 320 }}>
              <option>🌐 全球代理</option>
              <option>🍎 苹果服务</option>
              <option>📺 流媒体</option>
            </select>
          </CardSetting>

          <CardSetting
            title="base_url"
            desc="生成订阅链接时使用的基础地址，仅 scheme 和 host。Surge Managed Profile 会用到。"
          >
            <input className="b-mono" defaultValue="https://sub.example.com" style={{ ...inputB(), width: 360 }} />
            <div style={{ marginTop: 10, padding: '10px 14px', background: B.panelAlt, borderRadius: 8, fontSize: 12, color: B.textMuted }}>
              <span style={{ color: B.textDim, marginRight: 6 }}>预览：</span>
              <code className="b-mono" style={{ color: B.accent }}>https://sub.example.com/generate?format=clash&token=••••</code>
            </div>
          </CardSetting>

          <CardSetting
            title="Clash 模板"
            desc="生成 Clash Meta 配置时使用的基础模板。可填本地路径或 HTTP URL。"
          >
            <div style={{ display: 'flex', gap: 8 }}>
              <input className="b-mono" defaultValue="./templates/clash.yaml" style={inputB()} />
              <button style={btnB('secondary')}>预览</button>
            </div>
          </CardSetting>

          <CardSetting
            title="Surge 模板"
            desc="生成 Surge 配置时使用的基础模板。"
          >
            <div style={{ display: 'flex', gap: 8 }}>
              <input className="b-mono" defaultValue="./templates/surge.conf" style={inputB()} />
              <button style={btnB('secondary')}>预览</button>
            </div>
          </CardSetting>

          <CardSetting
            title="访问令牌"
            desc="生成订阅链接时附带的鉴权 token。变更后会让旧链接失效。"
          >
            <div style={{ display: 'flex', gap: 8 }}>
              <input className="b-mono" defaultValue="••••a83f7c2d••••" style={inputB()} />
              <button style={btnB('secondary')}>重置</button>
              <button style={btnB('secondary')}>显示</button>
            </div>
          </CardSetting>
        </div>
      </div>
    </ShellB>
  );
}

function CardSetting({ title, desc, children }) {
  return (
    <div style={{
      background: B.panel, border: `1px solid ${B.border}`, borderRadius: 12,
      padding: '20px 22px', boxShadow: '0 1px 2px rgba(0,0,0,0.03)',
    }}>
      <h3 style={{ margin: 0, fontSize: 15, fontWeight: 600 }}>{title}</h3>
      <div style={{ fontSize: 13, color: B.textMuted, marginTop: 4, marginBottom: 14, lineHeight: 1.55 }}>{desc}</div>
      {children}
    </div>
  );
}

Object.assign(window, { FiltersB, RoutingB, RulesetsB, RulesB, OtherB });
