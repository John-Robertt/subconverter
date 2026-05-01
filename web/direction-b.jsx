// Direction B — Modern SaaS / Tailwind-ish
// Generous whitespace, soft shadows, larger type, friendlier voice.
// Same three artboards: Sources, Groups, Generate.

const B = {
  bg: 'var(--b-bg)',
  panel: 'var(--b-panel)',
  panelAlt: 'var(--b-panel-alt)',
  border: 'var(--b-border)',
  text: 'var(--b-text)',
  textMuted: 'var(--b-text-muted)',
  textDim: 'var(--b-text-dim)',
  accent: 'var(--b-accent)',
  accentSoft: 'var(--b-accent-soft)',
  font: '"Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif',
  mono: '"JetBrains Mono", ui-monospace, monospace',
};

if (typeof document !== 'undefined' && !document.getElementById('dir-b-styles')) {
  const s = document.createElement('style');
  s.id = 'dir-b-styles';
  s.textContent = `
    .dir-b {
      --b-bg: #f8fafc;
      --b-panel: #ffffff;
      --b-panel-alt: #f1f5f9;
      --b-border: #e2e8f0;
      --b-text: #0f172a;
      --b-text-muted: #475569;
      --b-text-dim: #94a3b8;
      --b-accent: #6366f1;
      --b-accent-soft: rgba(99, 102, 241, 0.1);
      font-family: ${B.font};
      color: var(--b-text);
      font-size: 14px;
      line-height: 1.55;
      -webkit-font-smoothing: antialiased;
    }
    .dir-b.dark {
      --b-bg: #0b0f1a;
      --b-panel: #131826;
      --b-panel-alt: #1a2031;
      --b-border: #27304a;
      --b-text: #f1f5f9;
      --b-text-muted: #94a3b8;
      --b-text-dim: #64748b;
      --b-accent-soft: rgba(99, 102, 241, 0.18);
    }
    .dir-b *::-webkit-scrollbar { width: 10px; height: 10px; }
    .dir-b *::-webkit-scrollbar-thumb { background: var(--b-border); border-radius: 5px; }
    .dir-b *::-webkit-scrollbar-track { background: transparent; }
    .dir-b .b-mono { font-family: ${B.mono}; }
    .dir-b button { font-family: inherit; }
  `;
  document.head.appendChild(s);
}

function ShellB({ children, dark, accent, page = 'sources', topbar, loadingState }) {
  return (
    <div className={`dir-b${dark ? ' dark' : ''}`} style={{
      width: '100%', height: '100%', display: 'flex', background: B.bg,
      '--b-accent': accent, '--b-accent-soft': accent + '1a',
    }}>
      <SidebarB page={page} />
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        {topbar || <TopbarB page={page} loadingState={loadingState} />}
        <div style={{ flex: 1, overflow: 'hidden', minHeight: 0, background: B.bg }}>{children}</div>
      </div>
    </div>
  );
}

function SidebarB({ page }) {
  const items = [
    ['sources', '订阅来源', '7'],
    ['filters', '过滤器', null],
    ['groups', '节点分组', '6'],
    ['routing', '路由策略', '5'],
    ['rulesets', '规则集', null],
    ['rules', '内联规则', '23'],
    ['other', '其他配置', null],
    ['validate', '配置校验', '3'],
  ];
  const runtime = [
    ['preview', '节点预览'],
    ['grouppreview', '分组预览'],
    ['generate', '生成与下载'],
    ['health', '系统状态'],
  ];
  return (
    <div style={{
      width: 240, flex: '0 0 240px', borderRight: `1px solid ${B.border}`,
      background: B.panel, display: 'flex', flexDirection: 'column', padding: '20px 14px',
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '0 8px 18px' }}>
        <div style={{
          width: 32, height: 32, borderRadius: 9, background: `linear-gradient(135deg, ${'var(--b-accent)'} 0%, #8b5cf6 100%)`,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          color: '#fff', fontSize: 14, fontWeight: 700,
          boxShadow: '0 4px 12px rgba(99,102,241,0.25)',
        }}>S</div>
        <div>
          <div style={{ fontWeight: 600, fontSize: 14 }}>subconverter</div>
          <div style={{ fontSize: 11, color: B.textDim }}>v2.4.1</div>
        </div>
      </div>

      <div style={{ fontSize: 11, fontWeight: 600, color: B.textDim, padding: '6px 8px', textTransform: 'uppercase', letterSpacing: '0.06em' }}>配置</div>
      {items.map(([id, label, count]) => (
        <NavItemB key={id} id={id} label={label} count={count} active={page === id} icon={ICONS_B[id]} />
      ))}
      <div style={{ fontSize: 11, fontWeight: 600, color: B.textDim, padding: '14px 8px 6px', textTransform: 'uppercase', letterSpacing: '0.06em' }}>运行时</div>
      {runtime.map(([id, label]) => (
        <NavItemB key={id} id={id} label={label} active={page === id} icon={ICONS_B[id]} />
      ))}

      <div style={{
        marginTop: 'auto', padding: 12, borderRadius: 10, background: B.panelAlt,
        display: 'flex', alignItems: 'center', gap: 10,
      }}>
        <div style={{
          width: 8, height: 8, borderRadius: '50%', background: '#22c55e',
          boxShadow: '0 0 8px #22c55e',
        }}></div>
        <div style={{ flex: 1 }}>
          <div style={{ fontSize: 12, fontWeight: 500 }}>服务运行中</div>
          <div style={{ fontSize: 11, color: B.textDim }}>2 分钟前热重载</div>
        </div>
      </div>
    </div>
  );
}

const ICONS_B = {
  sources: <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round"><path d="M2 4.5h12M2 8h12M2 11.5h8"/></svg>,
  filters: <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round"><path d="M2 3h12l-4.5 6v4l-3 1.5V9L2 3z"/></svg>,
  groups: <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6"><rect x="2" y="2" width="5" height="5" rx="1"/><rect x="9" y="2" width="5" height="5" rx="1"/><rect x="2" y="9" width="5" height="5" rx="1"/><rect x="9" y="9" width="5" height="5" rx="1"/></svg>,
  routing: <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6"><circle cx="3" cy="8" r="2"/><circle cx="13" cy="3" r="2"/><circle cx="13" cy="13" r="2"/><path d="M5 8l6-5M5 8l6 5"/></svg>,
  rulesets: <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6"><path d="M3 2h7l3 3v9H3z"/><path d="M10 2v3h3"/></svg>,
  rules: <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round"><path d="M3 4h10M3 8h10M3 12h6"/></svg>,
  other: <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6"><circle cx="8" cy="8" r="2.5"/><path d="M8 2v1.5M8 12.5V14M2 8h1.5M12.5 8H14M3.8 3.8l1 1M11.2 11.2l1 1M3.8 12.2l1-1M11.2 4.8l1-1"/></svg>,
  validate: <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><path d="M8 2L2 4v4c0 3 2.5 5 6 6 3.5-1 6-3 6-6V4z"/><path d="M5.5 8l2 2 3-3.5"/></svg>,
  preview: <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6"><path d="M1 8s2.5-5 7-5 7 5 7 5-2.5 5-7 5-7-5-7-5z"/><circle cx="8" cy="8" r="2"/></svg>,
  grouppreview: <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6"><circle cx="4" cy="4" r="2"/><circle cx="12" cy="4" r="2"/><circle cx="8" cy="12" r="2"/><path d="M6 4h4M5 6l2 4M11 6l-2 4"/></svg>,
  generate: <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><path d="M8 2v8M4 6l4 4 4-4M2 13h12"/></svg>,
  health: <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><path d="M1 8h3l2-5 4 10 2-5h3"/></svg>,
};

function NavItemB({ id, label, count, active, icon }) {
  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 10,
      padding: '8px 10px', borderRadius: 8, cursor: 'pointer',
      background: active ? B.accentSoft : 'transparent',
      color: active ? B.accent : B.text,
      fontSize: 13, fontWeight: active ? 500 : 400,
    }}>
      <span style={{ display: 'inline-flex', opacity: active ? 1 : 0.7 }}>{icon}</span>
      <span>{label}</span>
      {count && <span style={{
        marginLeft: 'auto', fontSize: 11, padding: '1px 7px', borderRadius: 999,
        background: active ? 'var(--b-accent)' : B.panelAlt,
        color: active ? '#fff' : B.textMuted,
        fontWeight: 500,
      }}>{count}</span>}
    </div>
  );
}

function TopbarB({ page, title: tOverride, subtitle: sOverride, actions, loadingState }) {
  const titles = {
    sources: '订阅来源', groups: '节点分组', generate: '生成与下载',
    filters: '过滤器', routing: '路由策略', rulesets: '规则集',
    rules: '内联规则', other: '其他配置', validate: '配置校验',
    preview: '节点预览', grouppreview: '分组预览', health: '系统状态',
  };
  const subs = {
    sources: '管理上游订阅、单节点池与自定义代理',
    groups: '将节点按地区或属性聚合为可路由的分组',
    generate: '导出 Clash Meta 与 Surge 配置文件',
    filters: '用正则排除流量信息节点和广告条目',
    routing: '组装服务组，将分组、特殊关键字和规则集串起来',
    rulesets: '为每个服务组挂载远端规则列表',
    rules: '直接在配置里写的内联路由规则',
    other: 'fallback / base_url / 模板等基础设置',
    validate: '保存前的全面校验，错误会集中列出',
    preview: '查看所有来源拉取到的真实节点',
    grouppreview: '查看每个分组与服务组实际包含的节点',
    health: '后端进程、配置加载与热重载历史',
  };
  return (
    <div style={{
      padding: '20px 32px', borderBottom: `1px solid ${B.border}`,
      background: B.panel, display: 'flex', alignItems: 'center', gap: 16,
      flex: '0 0 auto',
    }}>
      <div>
        <h1 style={{ margin: 0, fontSize: 20, fontWeight: 600, letterSpacing: '-0.015em' }}>{tOverride || titles[page] || page}</h1>
        <div style={{ fontSize: 13, color: B.textMuted, marginTop: 2 }}>{sOverride || subs[page] || ''}</div>
      </div>
      <div style={{ marginLeft: 'auto', display: 'flex', gap: 8, alignItems: 'center' }}>
        {actions || (
          <>
            <span style={{ fontSize: 12, color: B.textDim, marginRight: 4 }}>config.yaml</span>
            {loadingState === 'validating' ? (
              <button style={{ ...btnB('secondary'), opacity: 0.7, cursor: 'not-allowed' }} disabled>
                <Spinner size={12} color={B.accent}/>
                <span style={{ marginLeft: 6 }}>校验中…</span>
              </button>
            ) : (
              <button style={btnB('secondary')}>校验</button>
            )}
            {loadingState === 'saving' ? (
              <button style={{ ...btnB('primary'), opacity: 0.7, cursor: 'not-allowed' }} disabled>
                <Spinner size={12} color="#fff"/>
                <span style={{ marginLeft: 6 }}>保存中…</span>
              </button>
            ) : (
              <button style={{ ...btnB('primary'), opacity: loadingState === 'validating' ? 0.5 : 1 }} disabled={loadingState === 'validating'}>保存并热重载</button>
            )}
          </>
        )}
      </div>
    </div>
  );
}

function btnB(variant) {
  const base = {
    height: 36, padding: '0 16px', borderRadius: 8, border: 'none',
    fontSize: 13, fontWeight: 500, cursor: 'pointer', display: 'inline-flex',
    alignItems: 'center', gap: 6,
  };
  if (variant === 'primary') return {
    ...base, background: B.accent, color: '#fff',
    boxShadow: '0 1px 2px rgba(0,0,0,0.05), 0 4px 12px rgba(99,102,241,0.25)',
  };
  if (variant === 'secondary') return {
    ...base, background: B.panel, color: B.text, border: `1px solid ${B.border}`,
    boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
  };
  return base;
}

// ---- Sources ----
function SourcesB(props) {
  return (
    <ShellB {...props} page="sources">
      <div style={{ height: '100%', overflow: 'auto', padding: '28px 32px' }}>
        {/* Stat row */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 14, marginBottom: 24 }}>
          <StatCardB label="订阅总数" value="7" sub="4 类来源" />
          <StatCardB label="活跃节点" value="19" sub="原始 21" />
          <StatCardB label="Surge-only" value="2" sub="Snell" />
          <StatCardB label="Clash-only" value="3" sub="VLESS" />
        </div>

        <SectionB
          title="SS 订阅"
          subtitle="Shadowsocks · 同时输出到 Clash 和 Surge"
          icon="🔗"
          count={SOURCES.ss.length}
        >
          {SOURCES.ss.map(s => <SourceCardB key={s.id} url={s.url} type="ss" />)}
          <AddButtonB label="添加 SS 订阅" />
        </SectionB>

        <SectionB
          title="Snell 节点池"
          subtitle="Surge 专属代理协议"
          icon="🛡️"
          count={SOURCES.snell.length}
          tag="仅 Surge"
          tagColor="#f59e0b"
        >
          {SOURCES.snell.map(s => <SourceCardB key={s.id} url={s.url} type="snell" />)}
          <AddButtonB label="添加 Snell 池" />
        </SectionB>

        <SectionB
          title="VLESS 节点池"
          subtitle="Reality / Vision 等高级特性"
          icon="🌀"
          count={SOURCES.vless.length}
          tag="仅 Clash"
          tagColor="#3b82f6"
        >
          {SOURCES.vless.map(s => <SourceCardB key={s.id} url={s.url} type="vless" />)}
          <AddButtonB label="添加 VLESS 池" />
        </SectionB>

        <SectionB
          title="自定义代理"
          subtitle="单节点直连，可链式中转"
          icon="⚙️"
          count={SOURCES.custom.length}
        >
          {SOURCES.custom.map(c => <CustomCardB key={c.id} c={c} />)}
          <AddButtonB label="添加自定义代理" />
        </SectionB>
      </div>
    </ShellB>
  );
}

function StatCardB({ label, value, sub }) {
  return (
    <div style={{
      background: B.panel, border: `1px solid ${B.border}`,
      borderRadius: 12, padding: '16px 18px',
      boxShadow: '0 1px 2px rgba(0,0,0,0.03)',
    }}>
      <div style={{ fontSize: 12, color: B.textMuted, marginBottom: 6 }}>{label}</div>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
        <span style={{ fontSize: 26, fontWeight: 600, letterSpacing: '-0.02em' }}>{value}</span>
        {sub && <span style={{ fontSize: 12, color: B.textDim }}>{sub}</span>}
      </div>
    </div>
  );
}

function SectionB({ title, subtitle, icon, count, tag, tagColor, children }) {
  return (
    <div style={{ marginBottom: 24 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12, padding: '0 4px' }}>
        <span style={{ fontSize: 18 }}>{icon}</span>
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <h3 style={{ margin: 0, fontSize: 15, fontWeight: 600 }}>{title}</h3>
            <span style={{
              fontSize: 11, padding: '2px 8px', borderRadius: 999,
              background: B.panelAlt, color: B.textMuted, fontWeight: 500,
            }}>{count}</span>
            {tag && <span style={{
              fontSize: 11, padding: '2px 8px', borderRadius: 999,
              background: tagColor + '1a', color: tagColor, fontWeight: 500,
            }}>{tag}</span>}
          </div>
          <div style={{ fontSize: 12, color: B.textMuted, marginTop: 1 }}>{subtitle}</div>
        </div>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>{children}</div>
    </div>
  );
}

function SourceCardB({ url, type }) {
  return (
    <div style={{
      background: B.panel, border: `1px solid ${B.border}`, borderRadius: 10,
      padding: '12px 16px', display: 'flex', alignItems: 'center', gap: 12,
      boxShadow: '0 1px 2px rgba(0,0,0,0.02)',
    }}>
      <span style={{ color: B.textDim, cursor: 'grab' }}>
        <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><circle cx="5" cy="4" r="1.2"/><circle cx="5" cy="8" r="1.2"/><circle cx="5" cy="12" r="1.2"/><circle cx="11" cy="4" r="1.2"/><circle cx="11" cy="8" r="1.2"/><circle cx="11" cy="12" r="1.2"/></svg>
      </span>
      <code className="b-mono" style={{
        flex: 1, fontSize: 12, color: B.textMuted, overflow: 'hidden',
        textOverflow: 'ellipsis', whiteSpace: 'nowrap',
      }}>{url}</code>
      <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: '#22c55e' }}>
        <span style={{ width: 6, height: 6, borderRadius: '50%', background: '#22c55e' }}></span>
        14 节点
      </span>
      <button style={iconBtnB()}>↻</button>
      <button style={iconBtnB()}>✎</button>
      <button style={{ ...iconBtnB(), color: '#ef4444' }}>✕</button>
    </div>
  );
}

function CustomCardB({ c }) {
  return (
    <div style={{
      background: B.panel, border: `1px solid ${B.border}`, borderRadius: 10,
      padding: '14px 16px', boxShadow: '0 1px 2px rgba(0,0,0,0.02)',
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <span style={{ color: B.textDim, cursor: 'grab' }}>
          <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><circle cx="5" cy="4" r="1.2"/><circle cx="5" cy="8" r="1.2"/><circle cx="5" cy="12" r="1.2"/><circle cx="11" cy="4" r="1.2"/><circle cx="11" cy="8" r="1.2"/><circle cx="11" cy="12" r="1.2"/></svg>
        </span>
        <span style={{ fontWeight: 500, fontSize: 14 }}>{c.name}</span>
        <span style={{
          fontSize: 11, padding: '2px 8px', borderRadius: 6,
          background: B.panelAlt, color: B.textMuted, fontFamily: B.mono,
        }}>{c.url.split('://')[0]}</span>
        {c.relay && <span style={{
          fontSize: 11, padding: '2px 8px', borderRadius: 6,
          background: '#a855f71a', color: '#9333ea',
        }}>↳ 中转 · {c.relay.strategy}</span>}
        <button style={{ ...iconBtnB(), marginLeft: 'auto' }}>✎</button>
        <button style={{ ...iconBtnB(), color: '#ef4444' }}>✕</button>
      </div>
      <code className="b-mono" style={{
        fontSize: 11, color: B.textDim, marginLeft: 24, marginTop: 6,
        display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
      }}>{c.url}</code>
      {c.relay && (
        <div style={{ marginLeft: 24, marginTop: 6, fontSize: 12, color: B.textMuted, display: 'flex', gap: 6, alignItems: 'center' }}>
          <span style={{ color: B.textDim }}>路由策略：</span>
          <code className="b-mono" style={{ background: B.panelAlt, padding: '1px 6px', borderRadius: 4, fontSize: 11 }}>{c.relay.name}</code>
        </div>
      )}
    </div>
  );
}

function AddButtonB({ label }) {
  return (
    <div style={{
      padding: '12px 16px', border: `1.5px dashed ${B.border}`, borderRadius: 10,
      color: B.accent, fontSize: 13, fontWeight: 500, cursor: 'pointer',
      textAlign: 'center', background: 'transparent',
    }}>+ {label}</div>
  );
}

function iconBtnB() {
  return {
    width: 28, height: 28, borderRadius: 6, border: 'none',
    background: 'transparent', color: B.textMuted, cursor: 'pointer', fontSize: 13,
  };
}

// ---- Groups ----
function GroupsB(props) {
  const [regex, setRegex] = React.useState('日本|东京|大阪|JP|🇯🇵');
  const [debounced, setDebounced] = React.useState(regex);
  React.useEffect(() => {
    const t = setTimeout(() => setDebounced(regex), 300);
    return () => clearTimeout(t);
  }, [regex]);
  const matches = React.useMemo(() => nodesForGroup(debounced), [debounced]);
  const valid = React.useMemo(() => { try { new RegExp(debounced); return true; } catch { return false; } }, [debounced]);

  return (
    <ShellB {...props} page="groups">
      <div style={{ height: '100%', display: 'flex', overflow: 'hidden' }}>
        <div style={{ flex: 1, overflow: 'auto', padding: '28px 32px', minWidth: 0 }}>
          {/* Group cards row */}
          <div style={{ display: 'flex', gap: 10, marginBottom: 24, flexWrap: 'wrap' }}>
            {GROUPS.map(g => {
              const m = nodesForGroup(g.regex).length;
              const active = g.id === 'g2';
              return (
                <div key={g.id} style={{
                  padding: '10px 14px', borderRadius: 10,
                  background: active ? B.accent : B.panel,
                  color: active ? '#fff' : B.text,
                  border: `1px solid ${active ? B.accent : B.border}`,
                  display: 'flex', alignItems: 'center', gap: 10, cursor: 'pointer',
                  boxShadow: active ? '0 4px 12px rgba(99,102,241,0.3)' : '0 1px 2px rgba(0,0,0,0.03)',
                  whiteSpace: 'nowrap', flex: '0 0 auto',
                }}>
                  <span style={{ color: active ? 'rgba(255,255,255,0.6)' : B.textDim, cursor: 'grab' }}>⠿</span>
                  <span style={{ fontWeight: 500, fontSize: 13, whiteSpace: 'nowrap' }}>{g.name}</span>
                  <span style={{
                    fontSize: 11, padding: '1px 7px', borderRadius: 999,
                    background: active ? 'rgba(255,255,255,0.2)' : B.panelAlt,
                    color: active ? '#fff' : B.textMuted,
                    whiteSpace: 'nowrap', flex: '0 0 auto',
                  }}>{m}</span>
                </div>
              );
            })}
            <div style={{
              padding: '10px 14px', borderRadius: 10, border: `1.5px dashed ${B.border}`,
              color: B.accent, fontSize: 13, cursor: 'pointer',
              whiteSpace: 'nowrap', flex: '0 0 auto',
            }}>+ 新建分组</div>
          </div>

          {/* Editor card */}
          <div style={{
            background: B.panel, border: `1px solid ${B.border}`, borderRadius: 14,
            padding: '24px 28px', boxShadow: '0 1px 3px rgba(0,0,0,0.04)', marginBottom: 16,
          }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 6 }}>
              <h2 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>编辑分组</h2>
              <span style={{ fontSize: 12, color: B.textDim, fontFamily: B.mono }}>group / g2</span>
            </div>
            <div style={{ fontSize: 13, color: B.textMuted, marginBottom: 20 }}>
              用正则匹配节点名，组成可被路由策略引用的逻辑分组。
            </div>

            <FieldB label="分组名称" hint="支持 emoji 前缀">
              <input defaultValue="🇯🇵 日本" style={inputB()} />
            </FieldB>

            <FieldB label="匹配正则" hint="300ms 后实时预览匹配结果">
              <input
                value={regex}
                onChange={e => setRegex(e.target.value)}
                className="b-mono"
                style={{ ...inputB(), borderColor: valid ? B.border : '#ef4444', color: valid ? B.text : '#ef4444' }}
              />
              {!valid && <div style={{ fontSize: 12, color: '#ef4444', marginTop: 6 }}>⚠ 正则语法错误</div>}
            </FieldB>

            <FieldB label="路由策略">
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
                <StratCardB title="select" desc="手动从分组中选择节点" />
                <StratCardB title="url-test" desc="自动测速选择延迟最低的节点" active />
              </div>
            </FieldB>
          </div>
        </div>

        {/* Live preview rail */}
        <div style={{
          width: 360, flex: '0 0 360px', borderLeft: `1px solid ${B.border}`,
          background: B.panel, display: 'flex', flexDirection: 'column', overflow: 'hidden',
        }}>
          <div style={{ padding: '20px 22px 16px', borderBottom: `1px solid ${B.border}` }}>
            <div style={{ fontSize: 12, fontWeight: 600, color: B.textDim, textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: 8 }}>实时匹配</div>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 10 }}>
              <span style={{ fontSize: 32, fontWeight: 600, color: valid ? B.accent : '#ef4444', letterSpacing: '-0.02em' }}>{matches.length}</span>
              <span style={{ fontSize: 13, color: B.textMuted }}>个节点 / {NODES.length} 总数</span>
            </div>
            <div style={{
              marginTop: 10, height: 4, borderRadius: 2, background: B.panelAlt, overflow: 'hidden',
            }}>
              <div style={{
                height: '100%', width: `${(matches.length / NODES.length) * 100}%`,
                background: B.accent, transition: 'width 0.3s',
              }}></div>
            </div>
          </div>
          <div style={{ flex: 1, overflow: 'auto', padding: '8px 0' }}>
            {NODES.map((n, i) => {
              let isMatch = false;
              try { isMatch = new RegExp(debounced).test(n.name); } catch {}
              const filtered = new RegExp(FILTERS.exclude).test(n.name);
              return (
                <div key={i} style={{
                  padding: '8px 22px', display: 'flex', alignItems: 'center', gap: 8,
                  fontSize: 13, opacity: filtered ? 0.4 : (isMatch ? 1 : 0.5),
                  background: isMatch && !filtered ? B.accentSoft : 'transparent',
                }}>
                  <span style={{
                    width: 6, height: 6, borderRadius: '50%',
                    background: isMatch && !filtered ? B.accent : B.border,
                  }}></span>
                  <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{n.name}</span>
                  {filtered && <span style={{ fontSize: 10, color: B.textDim }}>已过滤</span>}
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </ShellB>
  );
}

function FieldB({ label, hint, children }) {
  return (
    <div style={{ marginBottom: 20 }}>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 8 }}>
        <label style={{ fontSize: 13, fontWeight: 500 }}>{label}</label>
        {hint && <span style={{ fontSize: 12, color: B.textDim }}>{hint}</span>}
      </div>
      {children}
    </div>
  );
}

function inputB() {
  return {
    width: '100%', height: 38, padding: '0 12px', fontSize: 13,
    border: `1px solid ${B.border}`, borderRadius: 8,
    background: B.panel, color: B.text, fontFamily: 'inherit',
    boxSizing: 'border-box', outline: 'none',
  };
}

function StratCardB({ title, desc, active }) {
  return (
    <div style={{
      padding: '14px 16px', borderRadius: 10, cursor: 'pointer',
      border: `1.5px solid ${active ? B.accent : B.border}`,
      background: active ? B.accentSoft : B.panel,
    }}>
      <div style={{
        fontSize: 14, fontWeight: 600, fontFamily: B.mono,
        color: active ? B.accent : B.text, marginBottom: 4,
      }}>{title}</div>
      <div style={{ fontSize: 12, color: B.textMuted, lineHeight: 1.5 }}>{desc}</div>
    </div>
  );
}

// ---- Generate ----
function GenerateB(props) {
  return (
    <ShellB {...props} page="generate">
      <div style={{ height: '100%', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        {/* Stat cards */}
        <div style={{ padding: '20px 32px 0' }}>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 14, marginBottom: 16 }}>
            <StatCardB label="节点（过滤后）" value="19" sub="原始 21" />
            <StatCardB label="分组" value="6" sub="2 select · 4 url-test" />
            <StatCardB label="服务组" value="5" sub="带规则集" />
            <StatCardB label="规则总数" value="847" sub="含规则集" />
          </div>
        </div>

        {/* Side-by-side */}
        <div style={{ flex: 1, display: 'flex', gap: 14, padding: '4px 32px 24px', overflow: 'hidden', minHeight: 0 }}>
          <PreviewCardB
            title="Clash Meta"
            badge="YAML"
            badgeColor="#22c55e"
            subtitle="config.yaml · 24.3 KB"
            content={CLASH_PREVIEW}
            url="https://sub.example.com/generate?format=clash&token=••••"
            lang="clash"
          />
          <PreviewCardB
            title="Surge"
            badge="conf"
            badgeColor="#f59e0b"
            subtitle="config.conf · 18.7 KB"
            content={SURGE_PREVIEW}
            url="https://sub.example.com/generate?format=surge&token=••••"
            lang="surge"
            managed
          />
        </div>
      </div>
    </ShellB>
  );
}

function PreviewCardB({ title, badge, badgeColor, subtitle, content, url, lang, managed }) {
  return (
    <div style={{
      flex: 1, background: B.panel, border: `1px solid ${B.border}`, borderRadius: 14,
      display: 'flex', flexDirection: 'column', overflow: 'hidden', minWidth: 0,
      boxShadow: '0 1px 3px rgba(0,0,0,0.04)',
    }}>
      <div style={{
        padding: '16px 20px', borderBottom: `1px solid ${B.border}`,
        display: 'flex', alignItems: 'center', gap: 12,
      }}>
        <span style={{
          fontSize: 11, padding: '3px 9px', borderRadius: 6, fontWeight: 600,
          background: badgeColor + '1a', color: badgeColor, fontFamily: B.mono,
        }}>{badge}</span>
        <div>
          <div style={{ fontSize: 15, fontWeight: 600 }}>{title}</div>
          <div style={{ fontSize: 12, color: B.textDim, marginTop: 1 }}>{subtitle}</div>
        </div>
        <button style={{ ...btnB('primary'), marginLeft: 'auto', height: 32, fontSize: 12 }}>↓ 下载</button>
      </div>
      <div style={{
        padding: '12px 20px', background: B.panelAlt, borderBottom: `1px solid ${B.border}`,
        display: 'flex', alignItems: 'center', gap: 10,
      }}>
        <span style={{
          fontSize: 10, fontWeight: 600, color: B.textDim, fontFamily: B.mono,
          letterSpacing: '0.06em',
        }}>{managed ? 'MANAGED' : 'SUB URL'}</span>
        <code className="b-mono" style={{
          flex: 1, fontSize: 11, color: B.textMuted, overflow: 'hidden',
          textOverflow: 'ellipsis', whiteSpace: 'nowrap',
        }}>{url}</code>
        <button style={{ fontSize: 12, padding: '4px 10px', borderRadius: 6, border: `1px solid ${B.border}`, background: B.panel, cursor: 'pointer' }}>复制</button>
      </div>
      <div style={{ flex: 1, overflow: 'auto', minHeight: 0, background: B.panel }}>
        <CodeBlockB content={content} lang={lang} />
      </div>
    </div>
  );
}

function CodeBlockB({ content, lang }) {
  const lines = content.split('\n');
  return (
    <pre className="b-mono" style={{
      margin: 0, padding: '14px 0', fontSize: 12, lineHeight: 1.6,
      color: B.text, background: 'transparent',
    }}>
      {lines.map((line, i) => (
        <div key={i} style={{ display: 'flex', padding: '0 20px' }}>
          <span style={{
            display: 'inline-block', width: 28, color: B.textDim,
            textAlign: 'right', paddingRight: 12, userSelect: 'none', flexShrink: 0,
            fontSize: 11,
          }}>{i + 1}</span>
          <span style={{ flex: 1, whiteSpace: 'pre' }}>{highlightB(line, lang)}</span>
        </div>
      ))}
    </pre>
  );
}

function highlightB(line, lang) {
  if (!line.trim()) return line;
  if (line.trimStart().startsWith('#')) {
    return <span style={{ color: B.textDim, fontStyle: 'italic' }}>{line}</span>;
  }
  if (lang === 'surge' && /^\[.+\]$/.test(line.trim())) {
    return <span style={{ color: B.accent, fontWeight: 600 }}>{line}</span>;
  }
  const yamlMatch = line.match(/^(\s*[-]?\s*)([a-zA-Z_][\w-]*)(:)(.*)$/);
  if (yamlMatch && lang === 'clash') {
    return (
      <>
        <span>{yamlMatch[1]}</span>
        <span style={{ color: '#06b6d4' }}>{yamlMatch[2]}</span>
        <span>{yamlMatch[3]}</span>
        <span style={{ color: '#22c55e' }}>{yamlMatch[4]}</span>
      </>
    );
  }
  if (lang === 'surge') {
    const m = line.match(/^([^=]+)(=)(.*)$/);
    if (m) return (
      <>
        <span style={{ color: '#06b6d4' }}>{m[1]}</span>
        <span>{m[2]}</span>
        <span style={{ color: '#22c55e' }}>{m[3]}</span>
      </>
    );
  }
  return line;
}

Object.assign(window, {
  SourcesB, GroupsB, GenerateB,
  ShellB, SidebarB, TopbarB, NavItemB, ICONS_B,
  btnB, inputB, FieldB, SectionB, StatCardB, AddButtonB, iconBtnB,
  PreviewCardB, CodeBlockB, highlightB, B,
});
