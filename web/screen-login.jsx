// Login screen — Layout 1: centered card. 7 states driven by `state` prop.

const LOGIN_STATES = [
  'idle', 'validating', 'wrong-pwd', 'locked', 'redirecting', 'network-err', 'setup',
];

function LoginScreen({ dark, accent, state = 'idle' }) {
  // Inject B tokens via dir-b class for consistent theming
  const bg = dark ? '#0b1220' : '#f1f3f7';
  const cardBg = dark ? '#0f172a' : '#ffffff';
  const border = dark ? '#1e293b' : '#e2e8f0';
  const text = dark ? '#e2e8f0' : '#0f172a';
  const muted = dark ? '#94a3b8' : '#64748b';
  const dim = dark ? '#64748b' : '#94a3b8';
  const inputBg = dark ? '#0b1220' : '#ffffff';
  const inputBorder = dark ? '#1e293b' : '#cbd5e1';

  // Per-state config
  const isLocked = state === 'locked';
  const isWrong = state === 'wrong-pwd';
  const isValidating = state === 'validating';
  const isRedirecting = state === 'redirecting';
  const isNetErr = state === 'network-err';
  const isSetup = state === 'setup';
  const formDisabled = isLocked || isValidating || isRedirecting || isNetErr;

  return (
    <div className={dark ? 'dir-b dark' : 'dir-b'} style={{
      width: '100%', height: '100%', background: bg, color: text,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      fontFamily: 'system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif',
      position: 'relative', overflow: 'hidden',
    }}>
      {/* Soft accent gradient blob */}
      <div style={{
        position: 'absolute', top: '-20%', left: '50%', transform: 'translateX(-50%)',
        width: 800, height: 600, borderRadius: '50%',
        background: `radial-gradient(circle, ${accent}18, transparent 70%)`,
        pointerEvents: 'none',
      }}/>

      {/* Network error banner */}
      {isNetErr && (
        <div style={{
          position: 'absolute', top: 0, left: 0, right: 0,
          padding: '10px 24px', background: '#fef2f2', borderBottom: '1px solid #fecaca',
          display: 'flex', alignItems: 'center', gap: 10, fontSize: 13, color: '#991b1b',
        }}>
          <span style={{
            width: 8, height: 8, borderRadius: 4, background: '#dc2626',
            animation: 'b-pulse 2s ease-in-out infinite',
          }}/>
          <strong>后端不可达</strong>
          <span>· 无法连接 <code style={{ fontFamily: 'ui-monospace, monospace', fontSize: 12 }}>0.0.0.0:25500</code> · 请检查 subconverter 进程是否运行</span>
          <button style={{
            marginLeft: 'auto', padding: '4px 12px', borderRadius: 6,
            border: '1px solid #fca5a5', background: '#fff', color: '#991b1b',
            fontSize: 12, fontWeight: 500, cursor: 'pointer',
          }}>↻ 重试</button>
        </div>
      )}

      {/* Top-right theme switcher */}
      <div style={{
        position: 'absolute', top: 18, right: 24, display: 'flex', gap: 6,
        padding: 3, borderRadius: 999, background: cardBg, border: `1px solid ${border}`,
      }}>
        <button title="浅色" style={{
          width: 26, height: 26, borderRadius: 999, border: 'none',
          background: !dark ? accent : 'transparent', color: !dark ? '#fff' : muted,
          cursor: 'pointer', fontSize: 12,
        }}>☀</button>
        <button title="深色" style={{
          width: 26, height: 26, borderRadius: 999, border: 'none',
          background: dark ? accent : 'transparent', color: dark ? '#fff' : muted,
          cursor: 'pointer', fontSize: 12,
        }}>☾</button>
      </div>

      {/* Card */}
      <div style={{
        width: 380, background: cardBg, borderRadius: 16,
        border: `1px solid ${border}`,
        boxShadow: dark
          ? '0 24px 48px rgba(0,0,0,0.4), 0 0 0 1px rgba(255,255,255,0.04) inset'
          : '0 1px 2px rgba(15,23,42,0.04), 0 24px 48px rgba(15,23,42,0.08)',
        padding: '36px 36px 28px', position: 'relative', zIndex: 1,
      }}>
        {/* Logo wordmark */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 28, justifyContent: 'center' }}>
          <div style={{
            width: 32, height: 32, borderRadius: 8,
            background: `linear-gradient(135deg, ${accent}, ${accent}b3)`,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            color: '#fff', fontWeight: 700, fontSize: 16, letterSpacing: '-0.04em',
          }}>S</div>
          <span style={{
            fontSize: 18, fontWeight: 700, letterSpacing: '-0.02em', color: text,
          }}>subconverter</span>
        </div>

        {/* Setup mode header */}
        {isSetup && (
          <div style={{
            padding: '12px 14px', borderRadius: 10, marginBottom: 20,
            background: dark ? '#0c2818' : '#f0fdf4',
            border: `1px solid ${dark ? '#14532d' : '#bbf7d0'}`,
          }}>
            <div style={{ fontSize: 13, fontWeight: 600, color: dark ? '#86efac' : '#166534', marginBottom: 4 }}>
              ✓ 首次部署 · 设置管理员账号
            </div>
            <div style={{ fontSize: 11, color: dark ? '#4ade80' : '#15803d', lineHeight: 1.5 }}>
              检测到尚未初始化。请从服务日志复制 Setup Token，凭据将写入 <code style={{ fontFamily: 'ui-monospace, monospace' }}>auth.yaml</code>
            </div>
          </div>
        )}

        {!isSetup && (
          <div style={{ textAlign: 'center', marginBottom: 22 }}>
            <h1 style={{ margin: 0, fontSize: 18, fontWeight: 600, letterSpacing: '-0.01em' }}>登录管理后台</h1>
            <div style={{ fontSize: 12, color: muted, marginTop: 6 }}>使用管理员账号继续</div>
          </div>
        )}

        {/* Locked state banner */}
        {isLocked && (
          <div style={{
            padding: '12px 14px', borderRadius: 10, marginBottom: 18,
            background: dark ? '#2a0a0a' : '#fef2f2',
            border: `1px solid ${dark ? '#7f1d1d' : '#fecaca'}`,
          }}>
            <div style={{ fontSize: 13, fontWeight: 600, color: dark ? '#fca5a5' : '#991b1b', marginBottom: 4, display: 'flex', alignItems: 'center', gap: 6 }}>
              <span>🔒</span> 账号已临时锁定
            </div>
            <div style={{ fontSize: 11, color: dark ? '#f87171' : '#7f1d1d', lineHeight: 1.5 }}>
              连续 5 次登录失败 · 请于 <strong>14:32</strong> 后重试，或联系管理员重置
            </div>
          </div>
        )}

        {/* Username */}
        <div style={{ marginBottom: 14 }}>
          <label style={{ display: 'block', fontSize: 12, fontWeight: 500, color: muted, marginBottom: 6 }}>用户名</label>
          <input
            defaultValue={isSetup ? '' : 'admin'}
            disabled={formDisabled}
            placeholder={isSetup ? '设置管理员用户名' : ''}
            style={{
              width: '100%', boxSizing: 'border-box',
              height: 40, padding: '0 14px', borderRadius: 8,
              border: `1px solid ${inputBorder}`, background: inputBg, color: text,
              fontSize: 14, outline: 'none',
              opacity: formDisabled ? 0.6 : 1,
            }}
          />
        </div>

        {/* Setup token */}
        {isSetup && (
          <div style={{ marginBottom: 14 }}>
            <label style={{ display: 'block', fontSize: 12, fontWeight: 500, color: muted, marginBottom: 6 }}>Setup Token</label>
            <input
              type="password"
              disabled={formDisabled}
              placeholder="从服务启动日志复制"
              style={{
                width: '100%', boxSizing: 'border-box',
                height: 40, padding: '0 14px', borderRadius: 8,
                border: `1px solid ${inputBorder}`, background: inputBg, color: text,
                fontSize: 14, outline: 'none',
                opacity: formDisabled ? 0.6 : 1,
              }}
            />
            <div style={{ fontSize: 11, color: dim, marginTop: 6, lineHeight: 1.4 }}>
              未配置环境变量时，后端只会把一次性 token 打印到日志
            </div>
          </div>
        )}

        {/* Password */}
        <div style={{ marginBottom: 6 }}>
          <div style={{ display: 'flex', alignItems: 'baseline', marginBottom: 6 }}>
            <label style={{ fontSize: 12, fontWeight: 500, color: isWrong ? '#dc2626' : muted }}>
              {isSetup ? '设置密码' : '密码'}
            </label>
            {!isSetup && (
              <a style={{ marginLeft: 'auto', fontSize: 11, color: dim, textDecoration: 'none', cursor: 'pointer' }}>
                忘记密码？
              </a>
            )}
          </div>
          <div style={{ position: 'relative' }}>
            <input
              type="password"
              defaultValue={isWrong ? '••••••••' : (isSetup ? '' : '••••••••••')}
              disabled={formDisabled}
              placeholder={isSetup ? '至少 12 位，含字母与数字' : ''}
              style={{
                width: '100%', boxSizing: 'border-box',
                height: 40, padding: '0 38px 0 14px', borderRadius: 8,
                border: `1px solid ${isWrong ? '#ef4444' : inputBorder}`,
                background: inputBg, color: text, fontSize: 14, outline: 'none',
                boxShadow: isWrong ? '0 0 0 3px rgba(239,68,68,0.12)' : 'none',
                opacity: formDisabled ? 0.6 : 1,
              }}
            />
            <button
              tabIndex={-1}
              style={{
                position: 'absolute', right: 8, top: 8,
                width: 24, height: 24, border: 'none', background: 'transparent',
                color: dim, cursor: 'pointer', fontSize: 14,
              }}
              title="显示密码"
            >👁</button>
          </div>
          {isWrong && (
            <div style={{ fontSize: 12, color: '#dc2626', marginTop: 6, display: 'flex', alignItems: 'center', gap: 6 }}>
              <span>⚠</span>
              <span>用户名或密码错误 · 还可尝试 <strong>2</strong> 次</span>
            </div>
          )}
          {isSetup && (
            <div style={{ marginTop: 8, display: 'flex', gap: 4 }}>
              <div style={{ flex: 1, height: 3, borderRadius: 2, background: '#22c55e' }}/>
              <div style={{ flex: 1, height: 3, borderRadius: 2, background: '#22c55e' }}/>
              <div style={{ flex: 1, height: 3, borderRadius: 2, background: '#22c55e' }}/>
              <div style={{ flex: 1, height: 3, borderRadius: 2, background: dark ? '#1e293b' : '#e2e8f0' }}/>
              <span style={{ fontSize: 11, color: '#16a34a', fontWeight: 500, marginLeft: 4 }}>强</span>
            </div>
          )}
        </div>

        {/* Confirm pw for setup */}
        {isSetup && (
          <div style={{ margin: '14px 0 6px' }}>
            <label style={{ display: 'block', fontSize: 12, fontWeight: 500, color: muted, marginBottom: 6 }}>确认密码</label>
            <input
              type="password"
              style={{
                width: '100%', boxSizing: 'border-box',
                height: 40, padding: '0 14px', borderRadius: 8,
                border: `1px solid ${inputBorder}`, background: inputBg, color: text,
                fontSize: 14, outline: 'none',
              }}
            />
          </div>
        )}

        {/* Remember me */}
        {!isSetup && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, margin: '16px 0 18px' }}>
            <span style={{
              width: 16, height: 16, borderRadius: 4,
              border: `1.5px solid ${accent}`, background: accent,
              display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
              color: '#fff', fontSize: 11, fontWeight: 700,
            }}>✓</span>
            <label style={{ fontSize: 12, color: text, cursor: 'pointer' }}>
              记住我（保持登录 7 天）
            </label>
          </div>
        )}

        {/* Submit button */}
        <button
          disabled={formDisabled}
          style={{
            width: '100%', height: 42, marginTop: isSetup ? 18 : 0,
            borderRadius: 8, border: 'none', cursor: formDisabled ? 'not-allowed' : 'pointer',
            background: isLocked ? (dark ? '#1e293b' : '#cbd5e1') : accent,
            color: '#fff', fontSize: 14, fontWeight: 600,
            boxShadow: !formDisabled ? `0 1px 2px rgba(0,0,0,0.05), 0 8px 20px ${accent}40` : 'none',
            display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
            opacity: isLocked ? 0.7 : 1,
          }}
        >
          {isValidating && <Spinner size={14} color="#fff"/>}
          {isRedirecting && <Spinner size={14} color="#fff"/>}
          {isValidating && '正在验证…'}
          {isRedirecting && '登录成功 · 正在跳转…'}
          {isLocked && '账号锁定中'}
          {!isValidating && !isRedirecting && !isLocked && (isSetup ? '创建管理员并登录' : '登 录')}
        </button>

        {/* Caps Lock hint */}
        {state === 'idle' && false && (
          <div style={{ fontSize: 11, color: '#d97706', marginTop: 10, textAlign: 'center' }}>
            ⇪ Caps Lock 已开启
          </div>
        )}
      </div>

      {/* Footer */}
      <div style={{
        position: 'absolute', bottom: 16, left: 0, right: 0,
        display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 16,
        fontSize: 11, color: dim,
      }}>
        <span>subconverter v0.9.4</span>
        <span style={{ width: 3, height: 3, borderRadius: 2, background: dim }}/>
        <a style={{ color: dim, textDecoration: 'none', cursor: 'pointer' }}>文档</a>
        <span style={{ width: 3, height: 3, borderRadius: 2, background: dim }}/>
        <a style={{ color: dim, textDecoration: 'none', cursor: 'pointer' }}>GitHub</a>
      </div>
    </div>
  );
}

Object.assign(window, { LoginScreen, LOGIN_STATES });
