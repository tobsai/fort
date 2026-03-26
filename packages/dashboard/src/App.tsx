import { Routes, Route, Navigate } from "react-router-dom";
import TopBar from "./components/TopBar";
import SetupWizard from "./components/SetupWizard";
import ChatPage from "./pages/ChatPage";
import DashboardPage from "./pages/DashboardPage";
import AgentsPage from "./pages/AgentsPage";
import TasksPage from "./pages/TasksPage";
import SchedulesPage from "./pages/SchedulesPage";

function App() {
  return (
    <>
      <TopBar />
      <SetupWizard />
      <main className="app-content">
        <Routes>
          <Route path="/" element={<Navigate to="/chat" replace />} />
          <Route path="/chat" element={<ChatPage />} />
          <Route path="/chat/:agentId" element={<ChatPage />} />
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/agents" element={<AgentsPage />} />
          <Route path="/tasks" element={<TasksPage />} />
          <Route path="/schedules" element={<SchedulesPage />} />
          <Route path="*" element={<Navigate to="/chat" replace />} />
        </Routes>
      </main>
    </>
  );
}

export default App;
