import { useEffect, useState, type FormEvent } from "react";
import { App } from "../App";
import {
  getOperatorSession,
  operatorLogin,
  operatorLogout,
  type OperatorSession
} from "../lib/api";

export function OperatorGate() {
  const [session, setSession] = useState<OperatorSession | null>(null);
  const [controlKey, setControlKey] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let active = true;
    void getOperatorSession()
      .then((result) => { if (active) setSession(result); })
      .catch(() => { if (active) setError("Cannot reach the control API."); });
    return () => { active = false; };
  }, []);

  async function login(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const current = await operatorLogin(controlKey);
      setSession(current);
      setControlKey("");
    } catch {
      setError("Invalid operator key or too many attempts. Check your local first-boot credentials.");
    } finally {
      setBusy(false);
    }
  }

  async function logout() {
    setBusy(true);
    try {
      await operatorLogout();
      setSession({ enabled: true, authenticated: false });
    } catch {
      setError("Unable to sign out. Refresh the page to retry.");
    } finally {
      setBusy(false);
    }
  }

  if (!session) {
    return (
      <main className="operator-gate">
        <section className="operator-gate__panel" role="status">
          <img
            className="brand-lockup brand-lockup--login"
            src="/branding/bitriver-wordmark.svg"
            alt="BitRiver Studio OS"
            width="740"
            height="156"
          />
          <h1>Connecting to control API</h1>
          <p>{error || "Checking operator session..."}</p>
          {error && <button className="button button--secondary" onClick={() => window.location.reload()}>Retry</button>}
        </section>
      </main>
    );
  }

  if (session.enabled && !session.authenticated) {
    return (
      <main className="operator-gate">
        <form className="operator-gate__panel" onSubmit={(event) => void login(event)}>
          <img
            className="brand-lockup brand-lockup--login"
            src="/branding/bitriver-wordmark.svg"
            alt="BitRiver Studio OS"
            width="740"
            height="156"
          />
          <h1>Operator sign in</h1>
          <p>Enter the control key from the appliance's local first-boot notice. Do not send this key over public or unsecured networks.</p>
          <label htmlFor="operator-key">Operator control key</label>
          <input
            id="operator-key"
            type="password"
            autoComplete="off"
            value={controlKey}
            onChange={(event) => setControlKey(event.target.value)}
            required
            maxLength={256}
          />
          {error && <p role="alert" className="operator-gate__error">{error}</p>}
          <button type="submit" className="button button--primary" disabled={busy || !controlKey}>
            {busy ? "Signing in..." : "Sign in"}
          </button>
        </form>
      </main>
    );
  }

  return (
    <>
      {session.enabled && (
        <div className="operator-session">
          <img className="operator-session__mark" src="/branding/bitriver-mark.svg" alt="" aria-hidden="true" width="28" height="28" />
          <span>Operator session active</span>
          <button className="button button--ghost" type="button" onClick={() => void logout()} disabled={busy}>Sign out</button>
          {error && <span role="alert">{error}</span>}
        </div>
      )}
      <App />
    </>
  );
}
