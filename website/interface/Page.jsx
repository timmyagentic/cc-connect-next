import React, { useState } from "react";
import {
  MessagesSquare,
  Send,
  CornerDownRight,
  CheckCheck,
  Terminal,
  BookOpen,
  UserRound,
} from "lucide-react";
import { Button } from "@heroui/react/button";
import { Action, Choices, CopyAction } from "./ui.jsx";
import { dialogue } from "./dialogue.mjs";
const dual = (zh, en) => ({ zh, en });
export const site = {
  name: "CC Connect Next",
  lang: "zh",
  bilingual: true,
  nav: [
    [dual("如何使用", "How it works"), "#how"],
    [dual("使用指南", "Guides"), "/docs"],
    ["GitHub", "https://github.com/timmyagentic/cc-connect-next"],
  ],
};
const steps = [
  { id: "start", icon: Send },
  { id: "steer", icon: CornerDownRight },
  { id: "result", icon: CheckCheck },
];
export function Hero({ lang }) {
  const zh = lang === "zh";
  const t = dialogue[lang];
  const [current, setCurrent] = useState("start");
  const items = steps.map((step) => ({
    ...step,
    label: t["demoTab" + step.id],
  }));
  const idx = steps.findIndex((step) => step.id === current);
  return (
    <section className="pi-hero cc-hero" aria-labelledby="hero-title">
      <div className="pi-container cc-workspace">
        <div className="cc-directory">
          <Terminal size={30} aria-hidden="true" />
          <h1 id="hero-title">CC Connect Next</h1>
          <p>
            {zh
              ? "从飞书、Telegram 或 Slack 把任务交给自己电脑上的编程 Agent。"
              : "Talk to the coding agent on your own computer from Feishu, Telegram, or Slack."}
          </p>
          <CopyAction
            text="npm install -g cc-connect-next"
            lang={lang}
            label={zh ? "复制安装命令" : "Copy install command"}
          />
          <Action href="/docs#agent-install" icon={BookOpen}>
            {zh ? "打开安装指南" : "Installation guide"}
          </Action>
        </div>
        <article className="cc-conversation">
          <header>
            <MessagesSquare size={21} />
            <h2>{zh ? "登录页排查" : "Investigating a sign-in form"}</h2>
            <small>{t.demoLabel}</small>
          </header>
          <Choices
            label={zh ? "选择对话阶段" : "Conversation stage"}
            tabs={items}
            value={current}
            onChange={setCurrent}
          >
            {(item) => (
              <div className="cc-messages">
                <div className="cc-entry cc-user">
                  <UserRound size={17} aria-hidden="true" />
                  <div>
                    <strong>{t.userLabelstart}</strong>
                    <p>{t.requeststart}</p>
                  </div>
                </div>
                {item.id !== "start" && (
                  <div className="cc-entry cc-follow">
                    <CornerDownRight size={17} aria-hidden="true" />
                    <div>
                      <strong>{t.userLabelstart}</strong>
                      <p>{t["direction" + item.id]}</p>
                    </div>
                  </div>
                )}
                <div className="cc-entry cc-agent">
                  <Terminal size={17} aria-hidden="true" />
                  <div>
                    <strong>{t["agentLabel" + item.id]}</strong>
                    <p>{t[item.id + "Response"]}</p>
                    <small>{t[item.id + "Status"]}</small>
                  </div>
                </div>
              </div>
            )}
          </Choices>
          <div className="cc-chat-controls">
            <output aria-live="polite">
              {idx + 1} / 3 · {t["demoTab" + current]}
            </output>
            <Button
              className="cc-advance"
              onPress={() => setCurrent(steps[(idx + 1) % steps.length].id)}
            >
              {
                (zh
                  ? ["补充一句要求", "查看最终回复", "重新演示"]
                  : ["Add a follow-up", "See the result", "Replay"])[idx]
              }
            </Button>
          </div>
        </article>
      </div>
    </section>
  );
}
