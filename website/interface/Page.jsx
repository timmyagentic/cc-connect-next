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
import { Action, Choices, CopyAction, Mark, HeroCopy } from "./ui.jsx";
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
  const zh = lang === "zh",
    t = dialogue[lang],
    [current, setCurrent] = useState("start");
  const items = steps.map((s) => ({ ...s, label: t["demoTab" + s.id] }));
  const next = () =>
    setCurrent(steps[(steps.findIndex((s) => s.id === current) + 1) % 3].id);
  return (
    <section className="pi-hero cc-hero" aria-labelledby="hero-title">
      <div className="pi-container pi-split">
        <HeroCopy
          mark={<Mark Icon={Terminal}>CC Connect Next</Mark>}
          title={
            <>
              {t.heroFirst}
              <br />
              {t.heroSecond}
            </>
          }
          description={t.heroBody}
        >
          <div className="pi-actions">
            <Action href="/docs#agent-install" icon={BookOpen}>
              {zh ? "打开安装指南" : "Installation guide"}
            </Action>
          </div>
          <CopyAction
            text="npm install -g cc-connect-next"
            lang={lang}
            label={zh ? "复制安装命令" : "Copy install command"}
          />
        </HeroCopy>
        <article className="cc-chat">
          <div className="cc-chat-bar">
            <span>
              <MessagesSquare size={20} />{" "}
              {zh ? "本地 Agent 对话" : "Local agent conversation"}
            </span>
            <small>{t.demoLabel}</small>
          </div>
          <Choices
            label={zh ? "选择对话阶段" : "Conversation stage"}
            tabs={items}
            value={current}
            onChange={setCurrent}
          >
            {(item) => (
              <div className="cc-messages">
                <div className="cc-user">
                  <UserRound aria-hidden="true" size={15} />
                  <div>
                    <strong>{t.userLabelstart}</strong>
                    <p>{t.requeststart}</p>
                  </div>
                </div>
                {item.id !== "start" && (
                  <div className="cc-follow">
                    <CornerDownRight size={15} aria-hidden="true" />
                    <p>{t["direction" + item.id]}</p>
                  </div>
                )}
                <div className="cc-agent">
                  <Terminal aria-hidden="true" size={17} />
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
              {steps.findIndex((x) => x.id === current) + 1} / 3 ·{" "}
              {t["demoTab" + current]}
            </output>
            <Button className="cc-advance" onPress={next}>
              {
                (zh
                  ? ["补充一句要求", "查看最终回复", "重新演示"]
                  : ["Add a follow-up", "See the result", "Replay"])[
                  steps.findIndex((x) => x.id === current)
                ]
              }
            </Button>
          </div>
        </article>
      </div>
    </section>
  );
}
