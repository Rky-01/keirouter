import { Routes, Route } from "react-router-dom";
import { AuthGate } from "./components/AuthGate";
import { Layout } from "./components/Layout";
import { OverviewPage } from "./pages/Overview";
import { ProvidersPage } from "./pages/Providers";
import { ProviderDetailPage } from "./pages/ProviderDetail";
import { ChainsPage } from "./pages/Chains";
import { KeysPage } from "./pages/Keys";
import { BudgetsPage } from "./pages/Budgets";
import { SettingsPage } from "./pages/Settings";
import { EndpointsPage } from "./pages/Endpoints";
import { UsagePage } from "./pages/Usage";
import { QuotaPage } from "./pages/Quota";
import { CLIToolsPage } from "./pages/CLITools";
import { MediaProvidersPage } from "./pages/MediaProviders";
import { ProxyPoolsPage } from "./pages/ProxyPools";
import { SkillsPage } from "./pages/Skills";
import { ConsoleLogPage } from "./pages/ConsoleLog";

export function App() {
  return (
    <AuthGate>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<OverviewPage />} />
          <Route path="providers" element={<ProvidersPage />} />
          <Route path="providers/:id" element={<ProviderDetailPage />} />
          <Route path="endpoints" element={<EndpointsPage />} />
          <Route path="chains" element={<ChainsPage />} />
          <Route path="usage" element={<UsagePage />} />
          <Route path="quota" element={<QuotaPage />} />
          <Route path="cli-tools" element={<CLIToolsPage />} />
          <Route path="media" element={<MediaProvidersPage />} />
          <Route path="proxy-pools" element={<ProxyPoolsPage />} />
          <Route path="skills" element={<SkillsPage />} />
          <Route path="console" element={<ConsoleLogPage />} />
          <Route path="keys" element={<KeysPage />} />
          <Route path="budgets" element={<BudgetsPage />} />
          <Route path="settings" element={<SettingsPage />} />
        </Route>
      </Routes>
    </AuthGate>
  );
}