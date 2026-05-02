import { Navigate, NavLink, Route, Routes } from "react-router-dom";

type PageState = "ready" | "planned";

interface PageDefinition {
  path: string;
  label: string;
  section: "config" | "runtime" | "system";
  title: string;
  state: PageState;
}

const pages: PageDefinition[] = [
  { path: "/sources", label: "订阅来源", section: "config", title: "订阅来源", state: "ready" },
  { path: "/filters", label: "过滤器", section: "config", title: "过滤器", state: "planned" },
  { path: "/groups", label: "节点分组", section: "config", title: "节点分组", state: "planned" },
  { path: "/routing", label: "路由策略", section: "config", title: "路由策略", state: "planned" },
  { path: "/rulesets", label: "规则集", section: "config", title: "规则集", state: "planned" },
  { path: "/rules", label: "内联规则", section: "config", title: "内联规则", state: "planned" },
  { path: "/settings", label: "其他配置", section: "config", title: "其他配置", state: "planned" },
  { path: "/validate", label: "静态校验", section: "config", title: "静态校验", state: "planned" },
  { path: "/nodes", label: "节点预览", section: "runtime", title: "节点预览", state: "planned" },
  { path: "/preview/groups", label: "分组预览", section: "runtime", title: "分组预览", state: "planned" },
  { path: "/download", label: "生成下载", section: "runtime", title: "生成下载", state: "ready" },
  { path: "/status", label: "系统状态", section: "system", title: "系统状态", state: "ready" }
];

const sectionLabels: Record<PageDefinition["section"], string> = {
  config: "配置",
  runtime: "预览",
  system: "系统"
};

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPlaceholder />} />
      <Route path="/" element={<Navigate to="/sources" replace />} />
      <Route path="/*" element={<Shell />} />
    </Routes>
  );
}

function Shell() {
  return (
    <div className="app-shell">
      <aside className="sidebar" aria-label="主导航">
        <div className="brand">
          <span className="brand-mark" aria-hidden="true">S</span>
          <div>
            <div className="brand-name">subconverter</div>
            <div className="brand-subtitle">Admin</div>
          </div>
        </div>
        <nav className="nav-list">
          {pages.map((page) => (
            <NavLink
              key={page.path}
              to={page.path}
              className={({ isActive }) => (isActive ? "nav-link active" : "nav-link")}
            >
              <span>{page.label}</span>
              <small>{sectionLabels[page.section]}</small>
            </NavLink>
          ))}
        </nav>
      </aside>

      <main className="workspace">
        <header className="topbar">
          <div>
            <p className="eyebrow">M8</p>
            <h1>Web 镜像与 Compose 集成</h1>
          </div>
          <div className="status-row" aria-label="集成状态">
            <span className="status-badge good">SPA</span>
            <span className="status-badge neutral">nginx</span>
            <span className="status-badge neutral">Compose</span>
          </div>
        </header>

        <section className="surface">
          <Routes>
            {pages.map((page) => (
              <Route key={page.path} path={page.path.replace(/^\//, "")} element={<PlaceholderPage page={page} />} />
            ))}
            <Route path="*" element={<Navigate to="/sources" replace />} />
          </Routes>
        </section>
      </main>
    </div>
  );
}

function PlaceholderPage({ page }: { page: PageDefinition }) {
  return (
    <div className="page-panel">
      <div className="page-heading">
        <div>
          <p className="section-label">{sectionLabels[page.section]}</p>
          <h2>{page.title}</h2>
        </div>
        <span className={page.state === "ready" ? "state ready" : "state planned"}>
          {page.state === "ready" ? "已接入路由" : "后续里程碑"}
        </span>
      </div>
      <div className="placeholder-grid" aria-label={`${page.title} 占位区`}>
        <div className="placeholder-block wide" />
        <div className="placeholder-block" />
        <div className="placeholder-block" />
        <div className="placeholder-line" />
        <div className="placeholder-line short" />
      </div>
    </div>
  );
}

function LoginPlaceholder() {
  return (
    <main className="login-screen">
      <section className="login-panel" aria-labelledby="login-title">
        <span className="brand-mark large" aria-hidden="true">S</span>
        <div>
          <p className="section-label">认证</p>
          <h1 id="login-title">管理员登录</h1>
        </div>
        <div className="login-fields" aria-hidden="true">
          <span />
          <span />
          <button type="button">登录</button>
        </div>
      </section>
    </main>
  );
}

export default App;
