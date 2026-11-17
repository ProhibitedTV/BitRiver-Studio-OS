import React from "react";
import ReactDOM from "react-dom/client";

import { OperatorGate } from "./components/OperatorGate";
import "./styles.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <OperatorGate />
  </React.StrictMode>
);
