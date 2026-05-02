// State-driven overlay layer. Wraps a screen and renders the active interaction
// overlay based on `state`. Overlays are positioned absolute within the artboard.

function InteractionLayer({ state, children }) {
  return (
    <div style={{ position: 'relative', width: '100%', height: '100%', overflow: 'hidden' }}>
      {children}

      {/* 1b · Add modal */}
      {state === 'add-modal' && (
        <div style={{
          position: 'absolute', inset: 0, background: 'rgba(15,23,42,0.45)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          backdropFilter: 'blur(2px)', zIndex: 10,
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
      )}

      {/* 4a · Center confirm delete */}
      {state === 'confirm-delete' && (
        <div style={{
          position: 'absolute', inset: 0, background: 'rgba(15,23,42,0.45)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          backdropFilter: 'blur(2px)', zIndex: 10,
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
                  即将删除「<strong style={{ color: B.text }}>备用订阅</strong>」（4 个节点）。该操作不可撤销。
                </div>
              </div>
            </div>
            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 18 }}>
              <button style={btnB('secondary')}>取消</button>
              <button style={{ ...btnB('primary'), background: '#dc2626', borderColor: '#dc2626' }}>确认删除</button>
            </div>
          </div>
        </div>
      )}

      {/* 2a · Toast success */}
      {state === 'toast-success' && (
        <div style={{
          position: 'absolute', right: 24, bottom: 24,
          background: B.panel, border: `1px solid ${B.border}`, borderRadius: 10,
          padding: '12px 16px', boxShadow: '0 12px 32px rgba(15,23,42,0.15)',
          display: 'flex', alignItems: 'center', gap: 12, minWidth: 320,
          borderLeft: '4px solid #22c55e', zIndex: 10,
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
      )}

      {/* 2b · Toast error */}
      {state === 'toast-error' && (
        <div style={{
          position: 'absolute', right: 24, bottom: 24,
          background: B.panel, border: `1px solid ${B.border}`, borderRadius: 10,
          padding: '12px 16px', boxShadow: '0 12px 32px rgba(15,23,42,0.15)',
          display: 'flex', alignItems: 'flex-start', gap: 12, minWidth: 380,
          borderLeft: '4px solid #ef4444', zIndex: 10,
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
      )}
    </div>
  );
}

// State-aware artboard wrapper: applies overlay AND swaps topbar buttons to
// loading state when validating/saving.
function StatefulArtboard({ state, accent, dark, page, children }) {
  const sharedProps = { accent, dark };
  const isValidating = state === 'validating';
  const isSaving = state === 'saving';
  const showA8Drawer = state === 'a8-drawer' && page === 'validate';

  if (showA8Drawer) {
    return <DemoJumpDrawer {...sharedProps} />;
  }

  return (
    <InteractionLayer state={state}>
      {React.cloneElement(children, {
        ...sharedProps,
        loadingState: isValidating ? 'validating' : isSaving ? 'saving' : null,
      })}
    </InteractionLayer>
  );
}

Object.assign(window, { InteractionLayer, StatefulArtboard });
