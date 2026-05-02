import { Navigate, Route, Routes } from "react-router-dom";
import { ProtectedRoute } from "./app/ProtectedRoute";
import { AppShell } from "./layout/AppShell";
import { FiltersPage } from "./pages/FiltersPage";
import { GroupsPage } from "./pages/GroupsPage";
import { LoginPage } from "./pages/LoginPage";
import { NodesPage } from "./pages/NodesPage";
import { PlaceholderPage } from "./pages/PlaceholderPage";
import { RoutingPage } from "./pages/RoutingPage";
import { SourcesPage } from "./pages/SourcesPage";
import { StatusPage } from "./pages/StatusPage";

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<Navigate to="/sources" replace />} />
      <Route element={<ProtectedRoute />}>
        <Route element={<AppShell />}>
          <Route path="/sources" element={<SourcesPage />} />
          <Route path="/filters" element={<FiltersPage />} />
          <Route path="/groups" element={<GroupsPage />} />
          <Route path="/routing" element={<RoutingPage />} />
          <Route path="/nodes" element={<NodesPage />} />
          <Route path="/status" element={<StatusPage />} />
          <Route path="/rulesets" element={<PlaceholderPage title="规则集" />} />
          <Route path="/rules" element={<PlaceholderPage title="内联规则" />} />
          <Route path="/settings" element={<PlaceholderPage title="其他配置" />} />
          <Route path="/validate" element={<PlaceholderPage title="静态配置校验" />} />
          <Route path="/preview/groups" element={<PlaceholderPage title="分组预览" />} />
          <Route path="/download" element={<PlaceholderPage title="生成下载" />} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/sources" replace />} />
    </Routes>
  );
}

export default App;
