// Main app — chosen interaction patterns wired into real screens.

const TWEAK_DEFAULTS = /*EDITMODE-BEGIN*/{
  "theme": "light",
  "accent": "#6366f1",
  "loginState": "idle",
  "demoState": "idle"
}/*EDITMODE-END*/;

const ACCENT_PRESETS = [
  { value: '#6366f1', label: 'Indigo' },
  { value: '#0ea5e9', label: 'Sky' },
  { value: '#10b981', label: 'Emerald' },
  { value: '#f59e0b', label: 'Amber' },
  { value: '#ef4444', label: 'Red' },
  { value: '#a855f7', label: 'Purple' },
];

const LOGIN_STATE_LABELS = {
  'idle': 'idle · 默认',
  'validating': 'validating · 验证中',
  'wrong-pwd': 'wrong-pwd · 密码错误',
  'locked': 'locked · 临时锁定',
  'redirecting': 'redirecting · 跳转中',
  'network-err': 'network-err · 后端不可达',
  'setup': 'setup · 首次初始化',
};

// Locked patterns: 1b modal · 2a/2b toast · 3a button spinner · 4a center confirm · 5b A8 drawer

function App() {
  const [tweaks, setTweak] = useTweaks(TWEAK_DEFAULTS);
  const dark = tweaks.theme === 'dark';
  const accent = tweaks.accent;
  const state = tweaks.demoState || 'idle';
  const loginState = tweaks.loginState || 'idle';
  const p = { dark, accent, state };
  const Login = window.LoginScreen;
  const loginStateOptions = (window.LOGIN_STATES || []).map(value => ({
    value,
    label: LOGIN_STATE_LABELS[value] || value,
  }));

  const W = 1280, H = 820;

  return (
    <>
      <DesignCanvas>
        <DCSection
          id="auth"
          title="认证 · 登录页"
          subtitle="右下 Tweaks → 登录页状态 切换 idle / validating / wrong-pwd / locked / redirecting / network-err / setup"
        >
          <DCArtboard id="login" label="登录 / 首次 setup" width={W} height={H}>
            {Login ? <Login dark={dark} accent={accent} state={loginState} /> : null}
          </DCArtboard>
        </DCSection>

        <DCSection
          id="config"
          title="配置管理 · A 区"
          subtitle="A1 在 Tweaks 里切换交互状态：idle / 添加(modal) / 删除(确认) / 校验中 / 保存中 / Toast 成功 / Toast 失败"
        >
          <DCArtboard id="a1-sources" label="A1 订阅来源" width={W} height={H}>
            <StatefulArtboard state={state} accent={accent} dark={dark} page="sources">
              <SourcesB />
            </StatefulArtboard>
          </DCArtboard>
          <DCArtboard id="a2-filters" label="A2 过滤器" width={W} height={H}>
            <FiltersB {...p} />
          </DCArtboard>
          <DCArtboard id="a3-groups" label="A3 节点分组" width={W} height={H}>
            <GroupsB {...p} />
          </DCArtboard>
          <DCArtboard id="a4-routing" label="A4 路由策略" width={W} height={H}>
            <RoutingB {...p} />
          </DCArtboard>
          <DCArtboard id="a5-rulesets" label="A5 规则集" width={W} height={H}>
            <RulesetsB {...p} />
          </DCArtboard>
          <DCArtboard id="a6-rules" label="A6 内联规则" width={W} height={H}>
            <RulesB {...p} />
          </DCArtboard>
          <DCArtboard id="a7-other" label="A7 其他配置" width={W} height={H}>
            <OtherB {...p} />
          </DCArtboard>
          <DCArtboard id="a8-validate" label="A8 配置校验态（含 Drawer 直修）" width={W} height={H}>
            <StatefulArtboard state={state} accent={accent} dark={dark} page="validate">
              <ValidateB />
            </StatefulArtboard>
          </DCArtboard>
        </DCSection>

        <DCSection
          id="runtime"
          title="运行时预览 · B 区"
          subtitle="拉取真实数据，展示节点、分组成员、生成的配置"
        >
          <DCArtboard id="b1-preview" label="B1 节点预览" width={W} height={H}>
            <NodePreviewB {...p} />
          </DCArtboard>
          <DCArtboard id="b3-grouppreview" label="B3 分组预览" width={W} height={H}>
            <GroupPreviewB {...p} />
          </DCArtboard>
          <DCArtboard id="b4-generate" label="B4 / C 生成与下载" width={W} height={H}>
            <GenerateB {...p} />
          </DCArtboard>
        </DCSection>

        <DCSection
          id="system"
          title="系统状态 · D 区"
          subtitle="进程、配置、热重载历史"
        >
          <DCArtboard id="d-health" label="D 系统状态" width={W} height={H}>
            <HealthB {...p} />
          </DCArtboard>
        </DCSection>

        <DCSection
          id="patterns-locked"
          title="已选交互模式 · 参考"
          subtitle="1b 居中 Modal / 2a+2b Toast / 3a 按钮 Spinner / 4a 居中确认 / 5b A8 Drawer 直修"
        >
          <DCArtboard id="p-modal" label="1b · 添加订阅 Modal" width={W} height={H}>
            <DemoAddModal {...p} />
          </DCArtboard>
          <DCArtboard id="p-toast-ok" label="2a · Toast 成功" width={W} height={H}>
            <DemoToastSuccess {...p} />
          </DCArtboard>
          <DCArtboard id="p-toast-err" label="2b · Toast 失败" width={W} height={H}>
            <DemoToastError {...p} />
          </DCArtboard>
          <DCArtboard id="p-spinner" label="3a · 按钮 Spinner" width={W} height={H}>
            <DemoLoadingButton {...p} />
          </DCArtboard>
          <DCArtboard id="p-confirm" label="4a · 删除确认" width={W} height={H}>
            <DemoConfirmModal {...p} />
          </DCArtboard>
          <DCArtboard id="p-jump" label="5b · A8 Drawer 直修" width={W} height={H}>
            <DemoJumpDrawer {...p} />
          </DCArtboard>
        </DCSection>

        <DCPostIt x={40} y={60} w={280}>
          <strong>交互模式已锁定</strong>
          <br/><br/>
          · 1b 居中 Modal — 添加/编辑表单
          <br/>· 2a/2b Toast — 操作反馈
          <br/>· 3a 按钮 Spinner — 异步进行
          <br/>· 4a 居中确认 — 删除
          <br/>· 5b A8 Drawer — 直修字段
          <br/><br/>
          打开右下 <strong>Tweaks → 演示状态</strong> 在真实 A1/A8 上切换状态。下方留有参考 artboard。
        </DCPostIt>
      </DesignCanvas>

      <TweaksPanel title="Tweaks">
        <TweakSection title="登录页状态">
          <TweakSelect
            label="认证页"
            value={loginState}
            options={loginStateOptions}
            onChange={v => setTweak('loginState', v)}
          />
        </TweakSection>
        <TweakSection title="演示状态">
          <TweakSelect
            label="A1 / 顶栏状态"
            value={tweaks.demoState}
            options={[
              { value: 'idle', label: 'idle · 静态' },
              { value: 'add-modal', label: '添加订阅 Modal' },
              { value: 'confirm-delete', label: '删除确认' },
              { value: 'validating', label: '校验中（按钮 spinner）' },
              { value: 'saving', label: '保存中（按钮 spinner）' },
              { value: 'toast-success', label: 'Toast · 保存成功' },
              { value: 'toast-error', label: 'Toast · 校验失败' },
              { value: 'a8-drawer', label: 'A8 · Drawer 直修' },
            ]}
            onChange={v => setTweak('demoState', v)}
          />
        </TweakSection>
        <TweakSection title="主题">
          <TweakRadio
            label="明暗"
            value={tweaks.theme}
            options={[{ value: 'light', label: '亮' }, { value: 'dark', label: '暗' }]}
            onChange={v => setTweak('theme', v)}
          />
        </TweakSection>
        <TweakSection title="强调色">
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            {ACCENT_PRESETS.map(pr => (
              <button
                key={pr.value}
                onClick={() => setTweak('accent', pr.value)}
                title={pr.label}
                style={{
                  width: 28, height: 28, borderRadius: 8,
                  border: tweaks.accent === pr.value ? '2px solid #111' : '1px solid rgba(0,0,0,0.15)',
                  background: pr.value, cursor: 'pointer', padding: 0,
                  boxShadow: tweaks.accent === pr.value ? '0 0 0 2px #fff inset' : 'none',
                }}
              />
            ))}
          </div>
          <TweakColor label="自定义" value={tweaks.accent} onChange={v => setTweak('accent', v)} />
        </TweakSection>
      </TweaksPanel>
    </>
  );
}

ReactDOM.createRoot(document.getElementById('root')).render(<App />);
