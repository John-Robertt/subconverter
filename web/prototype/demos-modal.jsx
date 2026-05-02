// Interaction pattern demos — each artboard shows ONE state of ONE pattern,
// rendered statically (no real interactivity needed; the design canvas already
// gives focus/zoom). Side-by-side comparison of variants.

const W = 1280, H = 820;

// Reusable: a minimal "Sources" body with a list, used as the base for many demos.
function SourcesBodyDemo({ withSelected, deletingIdx }) {
  return (
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
        {[
          { url: 'https://nodes.example.org/sub/user_42/clash', name: '主订阅', count: 12 },
          { url: 'https://backup.example.io/api/sub?token=xxxx', name: '备用订阅', count: 4 },
          { url: 'https://provider.example.net/clash/UserABC', name: '次要源', count: 2 },
        ].map((s, i) => {
          const isDeleting = deletingIdx === i;
          const isSelected = withSelected === i;
          return (
            <div key={i} style={{
              padding: '14px 18px',
              borderBottom: i < 2 ? `1px solid ${B.border}` : 'none',
              background: isDeleting ? '#fef2f2' : isSelected ? B.accent + '10' : B.panel,
              borderLeft: isSelected ? `3px solid ${B.accent}` : '3px solid transparent',
              display: 'flex', alignItems: 'center', gap: 14,
            }}>
              <span style={{
                width: 8, height: 8, borderRadius: 4,
                background: isDeleting ? '#ef4444' : '#22c55e',
              }}/>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: 14, fontWeight: 500, color: isDeleting ? '#991b1b' : B.text }}>{s.name}</div>
                <code className="b-mono" style={{
                  fontSize: 11, color: B.textDim,
                  display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                  textDecoration: isDeleting ? 'line-through' : 'none',
                }}>{s.url}</code>
              </div>
              <span style={{
                fontSize: 11, padding: '2px 8px', borderRadius: 4,
                background: B.panelAlt, color: B.textMuted, fontFamily: B.mono,
              }}>{s.count} 节点</span>
              <button style={iconBtnB()}>✎</button>
              <button style={{ ...iconBtnB(), color: '#ef4444' }}>✕</button>
            </div>
          );
        })}
      </div>
    </div>
  );
}

// =============================================================
// 1. Modal/form 承载方式 — 添加订阅
// =============================================================

// 1a. 右侧 Drawer
function DemoAddDrawer(props) {
  return (
    <ShellB {...props} page="sources">
      <div style={{ height: '100%', position: 'relative', overflow: 'hidden' }}>
        <SourcesBodyDemo />
        {/* Drawer */}
        <div style={{
          position: 'absolute', inset: 0, background: 'rgba(15,23,42,0.18)',
        }}/>
        <div style={{
          position: 'absolute', top: 0, right: 0, bottom: 0, width: 460,
          background: B.panel, borderLeft: `1px solid ${B.border}`,
          boxShadow: '-12px 0 24px rgba(15,23,42,0.08)',
          display: 'flex', flexDirection: 'column',
        }}>
          <div style={{ padding: '20px 24px', borderBottom: `1px solid ${B.border}`, display: 'flex', alignItems: 'center' }}>
            <div>
              <h3 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>添加 SS 订阅</h3>
              <div style={{ fontSize: 12, color: B.textMuted, marginTop: 2 }}>支持 Clash / V2Ray / 通用 Base64</div>
            </div>
            <button style={{ ...iconBtnB(), marginLeft: 'auto' }}>✕</button>
          </div>
          <div style={{ flex: 1, padding: '20px 24px', overflow: 'auto' }}>
            <FieldB label="名称" hint="便于识别，可使用中文">
              <input defaultValue="主订阅" style={inputB()} />
            </FieldB>
            <FieldB label="订阅 URL" hint="粘贴整段 URL，会自动识别协议">
              <input
                className="b-mono"
                defaultValue="https://nodes.example.org/sub/user_42/clash"
                style={inputB()}
              />
              <div style={{
                marginTop: 8, padding: '8px 12px', borderRadius: 6,
                background: '#dcfce7', fontSize: 12, color: '#166534',
                display: 'flex', alignItems: 'center', gap: 8,
              }}>
                <span>✓</span> 已识别为 Clash 订阅 · 拉取到 12 个节点
              </div>
            </FieldB>
            <FieldB label="UA / User-Agent" hint="可选。某些机场要求特定 UA">
              <input className="b-mono" placeholder="ClashforWindows/0.20.39" style={inputB()} />
            </FieldB>
            <FieldB label="自动刷新" hint="保持订阅最新">
              <select style={inputB()}>
                <option>每 6 小时</option>
                <option>每 1 小时</option>
                <option>不自动刷新</option>
              </select>
            </FieldB>
          </div>
          <div style={{ padding: '14px 24px', borderTop: `1px solid ${B.border}`, display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
            <button style={btnB('secondary')}>取消</button>
            <button style={btnB('primary')}>保存订阅</button>
          </div>
        </div>
      </div>
    </ShellB>
  );
}

// 1b. 居中 Modal
function DemoAddModal(props) {
  return (
    <ShellB {...props} page="sources">
      <div style={{ height: '100%', position: 'relative', overflow: 'hidden' }}>
        <SourcesBodyDemo />
        <div style={{
          position: 'absolute', inset: 0, background: 'rgba(15,23,42,0.45)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          backdropFilter: 'blur(2px)',
        }}>
          <div style={{
            width: 520, background: B.panel, borderRadius: 14,
            boxShadow: '0 24px 48px rgba(15,23,42,0.25)',
            display: 'flex', flexDirection: 'column', maxHeight: '85%',
          }}>
            <div style={{ padding: '20px 24px', borderBottom: `1px solid ${B.border}` }}>
              <h3 style={{ margin: 0, fontSize: 17, fontWeight: 600 }}>添加 SS 订阅</h3>
              <div style={{ fontSize: 12, color: B.textMuted, marginTop: 4 }}>支持 Clash / V2Ray / 通用 Base64</div>
            </div>
            <div style={{ padding: '20px 24px', overflow: 'auto' }}>
              <FieldB label="名称">
                <input defaultValue="主订阅" style={inputB()} />
              </FieldB>
              <FieldB label="订阅 URL">
                <input className="b-mono" defaultValue="https://nodes.example.org/sub/user_42/clash" style={inputB()} />
                <div style={{
                  marginTop: 8, padding: '8px 12px', borderRadius: 6,
                  background: '#dcfce7', fontSize: 12, color: '#166534',
                }}>✓ 已识别为 Clash 订阅 · 12 节点</div>
              </FieldB>
              <FieldB label="自动刷新">
                <select style={inputB()}><option>每 6 小时</option></select>
              </FieldB>
            </div>
            <div style={{ padding: '14px 24px', borderTop: `1px solid ${B.border}`, display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
              <button style={btnB('secondary')}>取消</button>
              <button style={btnB('primary')}>保存订阅</button>
            </div>
          </div>
        </div>
      </div>
    </ShellB>
  );
}

// 1c. 行内展开
function DemoAddInline(props) {
  return (
    <ShellB {...props} page="sources">
      <div style={{ padding: '24px 32px' }}>
        <div style={{ display: 'flex', alignItems: 'center', marginBottom: 16 }}>
          <h2 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>SS 订阅</h2>
          <span style={{
            fontSize: 11, padding: '2px 8px', borderRadius: 999, marginLeft: 10,
            background: B.panelAlt, color: B.textMuted, fontWeight: 600, fontFamily: B.mono,
          }}>3</span>
        </div>
        <div style={{
          background: B.panel, border: `1px solid ${B.border}`, borderRadius: 12, overflow: 'hidden',
        }}>
          <div style={{ padding: '14px 18px', borderBottom: `1px solid ${B.border}`, display: 'flex', alignItems: 'center', gap: 14 }}>
            <span style={{ width: 8, height: 8, borderRadius: 4, background: '#22c55e' }}/>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: 14, fontWeight: 500 }}>主订阅</div>
              <code className="b-mono" style={{ fontSize: 11, color: B.textDim, display: 'block' }}>https://nodes.example.org/sub/user_42/clash</code>
            </div>
            <span style={{ fontSize: 11, padding: '2px 8px', borderRadius: 4, background: B.panelAlt, color: B.textMuted, fontFamily: B.mono }}>12 节点</span>
            <button style={iconBtnB()}>✎</button>
            <button style={{ ...iconBtnB(), color: '#ef4444' }}>✕</button>
          </div>

          {/* Inline expanded editor */}
          <div style={{
            padding: '20px 24px', background: B.accent + '08',
            borderBottom: `1px solid ${B.border}`,
            borderLeft: `3px solid ${B.accent}`,
          }}>
            <div style={{ fontSize: 12, fontWeight: 600, color: B.accent, textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 14 }}>
              新建订阅
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 2fr', gap: 12, marginBottom: 12 }}>
              <input placeholder="名称" style={inputB()} autoFocus />
              <input className="b-mono" placeholder="https://…" style={inputB()} />
            </div>
            <div style={{ display: 'flex', gap: 8 }}>
              <button style={btnB('primary')}>保存</button>
              <button style={btnB('secondary')}>取消</button>
              <span style={{ marginLeft: 'auto', fontSize: 12, color: B.textDim, alignSelf: 'center' }}>
                Esc 取消 · ⌘ + Enter 保存
              </span>
            </div>
          </div>

          <div style={{ padding: '14px 18px', display: 'flex', alignItems: 'center', gap: 14 }}>
            <span style={{ width: 8, height: 8, borderRadius: 4, background: '#22c55e' }}/>
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: 14, fontWeight: 500 }}>备用订阅</div>
              <code className="b-mono" style={{ fontSize: 11, color: B.textDim }}>https://backup.example.io/api/sub?token=xxxx</code>
            </div>
            <span style={{ fontSize: 11, padding: '2px 8px', borderRadius: 4, background: B.panelAlt, color: B.textMuted, fontFamily: B.mono }}>4 节点</span>
            <button style={iconBtnB()}>✎</button>
            <button style={{ ...iconBtnB(), color: '#ef4444' }}>✕</button>
          </div>
        </div>
      </div>
    </ShellB>
  );
}

Object.assign(window, { DemoAddDrawer, DemoAddModal, DemoAddInline, SourcesBodyDemo });
