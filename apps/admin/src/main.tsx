import "@arco-design/web-react/dist/css/arco.css";
import React from "react";
import ReactDOM from "react-dom/client";
import { ConfigProvider } from "@arco-design/web-react";
import zhCN from "@arco-design/web-react/es/locale/zh-CN";
import { BrowserRouter } from "react-router-dom";
import { AdminRouter } from "./router";
import "./styles.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ConfigProvider
      locale={zhCN}
      size="default"
      autoInsertSpaceInButton={false}
      componentConfig={{
        Button: { shape: "square" },
        Card: { bordered: true },
        Table: { borderCell: false }
      }}
    >
      <BrowserRouter>
        <AdminRouter />
      </BrowserRouter>
    </ConfigProvider>
  </React.StrictMode>
);
