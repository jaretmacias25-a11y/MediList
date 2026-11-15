import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ClerkProvider } from "@clerk/react";
import App from "./App";
import { clerkEnabled, clerkPublishableKey } from "@/lib/clerk";
import "./index.css";

const app = clerkEnabled ? (
  <ClerkProvider
    publishableKey={clerkPublishableKey}
    appearance={{ variables: { colorPrimary: "#2563eb" } }}
  >
    <App />
  </ClerkProvider>
) : (
  <App />
);

createRoot(document.getElementById("root")!).render(
  <StrictMode>{app}</StrictMode>,
);
