import { Navigate, Route, Routes } from "react-router-dom";
import { ProtectedRoute } from "./app/ProtectedRoute";
import { AppShell } from "./layout/AppShell";
import { DownloadPage } from "./pages/DownloadPage";
import { FiltersPage } from "./pages/FiltersPage";
import { GroupPreviewPage } from "./pages/GroupPreviewPage";
import { GroupsPage } from "./pages/GroupsPage";
import { LoginPage } from "./pages/LoginPage";
import { NodesPage } from "./pages/NodesPage";
import { RoutingPage } from "./pages/RoutingPage";
import { RulesetsPage } from "./pages/RulesetsPage";
import { RulesPage } from "./pages/RulesPage";
import { SettingsPage } from "./pages/SettingsPage";
import { SourcesPage } from "./pages/SourcesPage";
import { StatusPage } from "./pages/StatusPage";
import { ValidatePage } from "./pages/ValidatePage";

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
          <Route path="/rulesets" element={<RulesetsPage />} />
          <Route path="/rules" element={<RulesPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/validate" element={<ValidatePage />} />
          <Route path="/preview/groups" element={<GroupPreviewPage />} />
          <Route path="/download" element={<DownloadPage />} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/sources" replace />} />
    </Routes>
  );
}

export default App;
