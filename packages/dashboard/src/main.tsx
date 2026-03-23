import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { FortSocketProvider } from "./contexts/FortSocketContext";
import App from "./App";
import "./styles/global.css";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <BrowserRouter>
      <FortSocketProvider>
        <App />
      </FortSocketProvider>
    </BrowserRouter>
  </React.StrictMode>,
);
