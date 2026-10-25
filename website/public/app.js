export const translations = {
  "zh-CN": {
    navHow: "如何使用",
    navDocs: "使用指南",
    footerTag: "本地执行，随处连接。",
    docsLink: "文档",
    releases: "更新日志",
    credit: "源自 CC Connect，保留上游历史与 MIT 许可。",
    skip: "跳到正文",
    eyebrow: "本地 AI 编程，随处接着聊",
    heroFirst: "Agent 在电脑，",
    heroSecond: "你在任何地方。",
    heroBody:
      "在飞书、Telegram 或 Slack 里，继续和自己的编程 Agent 协作。\n代码与执行环境留在你的机器，消息把你们连起来。",
    copy: "复制",
    start: "安装与接入指南",
    platformNote: "macOS · Linux · Windows · 开源",
    demoHeading: "从一句话，到继续推进。",
    demoTabstart: "发起任务",
    demoTabsteer: "中途补充",
    demoTabresult: "查看结果",
    demoLabel: "交互示例 · 不连接你的机器",
    userLabelstart: "你",
    requeststart: "看看登录页的表单报错，先定位原因。",
    agentLabelstart: "你的本地 Agent",
    startResponse: "已收到。接下来读取表单代码，检查校验和错误提示。",
    startStatus: "处理中 · 在本地工作区执行",
    userLabelsteer: "你",
    requeststeer: "看看登录页的表单报错，先定位原因。",
    directionsteer: "补充：保留现有布局，只修输入校验。",
    agentLabelsteer: "你的本地 Agent",
    steerResponse: "已把补充要求并入当前任务，保留布局，只调整校验逻辑。",
    steerStatus: "示例：Codex 原生 steer · 其他后端可能排队",
    userLabelresult: "你",
    requestresult: "看看登录页的表单报错，先定位原因。",
    directionresult: "补充：保留现有布局，只修输入校验。",
    agentLabelresult: "你的本地 Agent",
    resultResponse:
      "已修复：空输入会显示提示，有效输入继续提交。原有布局保持不变。",
    resultStatus: "示例结果 · 可继续追问、检查差异或安排下一步",
    demoFoot: "示例展示消息流转；实际能力取决于所选 Agent、平台和配置。",
    howEyebrow: "熟悉的工具，新的连接方式",
    howTitle: "从办公桌，到你手边。",
    howBody: "一个 Go 二进制，连接聊天平台与本地编程 Agent。",
    stepTitle01: "Agent 跑在本地",
    stepBody01:
      "使用已经安装并登录的 Codex、Claude Code 等 CLI，在你的工作区执行。",
    stepTitle02: "消息送到身边",
    stepBody02:
      "绑定飞书、Telegram、Slack 等平台，发消息、收进展，继续同一段协作。",
    stepTitle03: "把权限握在手里",
    stepBody03:
      "按项目配置工作区与访问范围。能力和权限依所选 Agent 与平台而定。",
    feishuEyebrow: "为飞书深入打磨",
    feishuTitle: "消息背后，\n是完整的协作。",
    feishuBody: "流式回答、权限确认和话题隔离，让长任务也能在聊天里清楚推进。",
    feishuLink: "查看飞书接入指南",
    cardsTitle: "一个回答，持续更新",
    cardsBody: "用交互卡片呈现进度，支持的操作就在消息里完成。",
    topicsTitle: "话题各自专注",
    topicsBody: "真实话题可以拥有独立会话与工作区绑定，减少上下文混在一起。",
    steerTitle: "任务进行中，也能补充",
    steerBody:
      "Codex 原生 steer 可并入当前回合；其他 Agent 按实际能力回退排队。",
    integrationTitle: "用你熟悉的组合。",
    integrationBody: "先选 Agent，再选聊天平台。完整支持范围与限制见项目文档。",
    agentsLabel: "本地编程 Agent",
    platformsLabel: "聊天平台",
    allIntegrations: "查看全部集成与能力",
    startTitle: "从这里，连起来。",
    startBody: "全新安装、接入飞书，或从官方 CC Connect 迁移。",
    pathTitleinstall: "第一次使用",
    pathBodyinstall: "复制完整安装 Prompt，让 Agent 帮你配置、验证首条回复，再启动后台服务。",
    pathTitlefeishu: "接入飞书",
    pathBodyfeishu: "配置机器人并扫码授权；已有项目的工作目录需在配置文件中填写。",
    pathTitlemigration: "从 CC Connect 迁移",
    pathBodymigration: "先用 dry-run 查看迁移计划，再按文档决定是否切换服务。",
    faqTitle: "还有几个常见问题。",
    faqQlocal: "官网会连接我的电脑吗？",
    faqAlocal:
      "不会。官网提供介绍、交互示例与安装说明。实际连接由你在自己机器上安装和配置的 CC Connect Next 完成。",
    faqQpublic: "需要公网 IP 吗？",
    faqApublic:
      "飞书使用 WebSocket 长连接，不需要公网 IP、域名或证书。其他平台的接入要求见对应指南。",
    faqQprivacy: "数据会发往哪里？",
    faqAprivacy:
      "执行环境在本地；消息经过所选聊天平台，Agent 请求会发往其配置的服务提供方。本地执行并不代表完全离线。",
    faqQcost: "CC Connect Next 免费吗？",
    faqAcost:
      "代码采用 MIT 许可。Agent 订阅、模型 API 与聊天平台可能产生各自的费用。",
    finalTitle: "下一个想法，\n就在下一条消息里。",
    finalCta: "开始连接",
    agentInstallEyebrow: "让 Agent 帮你装好",
    agentInstallTitle: "从安装，\n到第一条回复。",
    agentInstallBody: "复制完整 Prompt，交给 Codex、Claude Code 等本地编码 Agent。它会处理配置与验收；登录、扫码和必要授权由你完成。",
    agentInstallCopy: "复制安装 Prompt",
    agentInstallDetails: "查看完整 Prompt",
    installPrompt: "请从 https://github.com/timmyagentic/cc-connect-next 安装已发布稳定版，按首次安装流程接入飞书/Lark。\n先确认操作系统、CPU 架构、Agent CLI 及登录状态和项目绝对路径；优先复用当前 Agent 与项目，\n缺少必要信息再问我。检查并保留已有 CC Connect/Next 配置和服务，不执行迁移或覆盖。\n安装后选择明确的配置绝对路径；文件不存在时运行 `cc-connect-next --config <配置路径>` 生成配置，\n由你填写真实 work_dir、Agent 类型和对应选项，保持项目名一致，不让我手工编辑配置。\n执行 `cc-connect-next feishu setup --config <配置路径> --project <项目名>`，展示二维码并保持\n进程等待我扫码。接受推荐配置，并按提示确认官方 lark-cli companion；非交互执行时使用\n`--recommended --lark-cli`，保留已有 profile，让同一机器人作为默认 bot profile。\n执行 `cc-connect-next doctor --config <配置路径>`，再用\n`cc-connect-next --config <配置路径>` 前台启动；引导我在机器人私聊发送 /version、/whoami\n和一条普通提问，核对回复并按实际用户 ID 配置访问范围。\n验证后停止本次前台进程，执行 `cc-connect-next daemon install --config <配置路径>`，检查\n`cc-connect-next daemon status` 的服务状态、Runtime 和平台就绪状态，并验证后台仍能回复。\n报告版本、配置路径、work_dir 和验证结果；需要我登录、扫码或发布应用时再提示，未完成的\n真实消息验证明确标为 UNVERIFIED。",
    manualTitle: "手动安装，按这五步走。",
    prepareTitle: "先准备可用的 Agent",
    prepareBody: "安装并登录一个受支持的 Agent CLI，确认它能在本机回答；选择真实存在的项目目录，并记下绝对路径。还没有 Agent 时先完成它的安装与登录。",
    verifyTitle: "前台启动，收到真实回复",
    verifyBody: "doctor 只检查环境，不连接平台。通过后前台启动，在机器人私聊发送 /version、/whoami 和一条普通提问，确认 Agent 回答；按实际用户 ID 设置访问范围。",
    backgroundTitle: "转到后台，再验证一次",
    backgroundBody: "先停止本次启动的前台进程，再安装后台服务。核对服务、Runtime 和平台就绪状态，然后再次发送普通提问，确认关闭终端后仍能回答。",
    migrationEyebrow: "已有官方 CC Connect？",
    docsEyebrow: "使用指南",
    docsTitle: "把你的 Agent 连起来。",
    docsIntro:
      "把完整安装 Prompt 交给能执行本机命令的 Agent，或按下面五步手动完成。已有配置和服务应先检查并保留。",
    docsHeadinginstall: "安装并填写配置",
    docsBodyinstall:
      "以下命令以 macOS / Linux 的默认配置路径为例；Windows 请使用对应的绝对路径。首次运行生成模板后退出是正常行为。填写真实 work_dir、Agent 类型和项目名，并替换所有 REPLACE 占位值；后续始终使用同一个配置文件。",
    docsCopyinstall: "复制命令",
    docsReadinstall: "查看完整首次安装说明",
    docsHeadingfeishu: "接入飞书",
    docsBodyfeishu:
      "使用配置里的同一项目名完成机器人设置与扫码，可选择官方 lark-cli companion。feishu setup 配置机器人，不会替已有项目填写 work_dir。",
    docsCopyfeishu: "复制命令",
    docsReadfeishu: "阅读完整指南",
    docsHeadingmigration: "先评估迁移，再切换服务",
    docsBodymigration:
      "dry-run 只报告计划。实际 migrate --switch 会切换服务运行与自启动状态；先阅读安装文档中的备份、兼容性和回滚说明。",
    docsCopymigration: "复制命令",
    docsReadmigration: "阅读完整指南",
    nextTitle: "接着了解",
    usage: "聊天命令与日常使用",
    config: "完整配置参考",
    manifest: "Agent 能力与权限",
    issues: "问题反馈",
    metaHome: "CC Connect Next — 你的本地 Agent，随处连接",
    metaDocs: "安装与接入 — CC Connect Next",
    meta404: "页面未找到 — CC Connect Next",
    descriptionHome:
      "通过飞书、Telegram、Slack 等聊天平台使用自己机器上的 Codex、Claude Code 等编程 Agent。",
    descriptionDocs: "用完整安装 Prompt 或五步指南，完成 Agent 准备、项目配置、飞书连接、首条回复和后台运行。",
    description404: "请求的页面不存在。",
  },
  en: {
    navHow: "How it works",
    navDocs: "Get started",
    footerTag: "Run locally. Connect from anywhere.",
    docsLink: "Documentation",
    releases: "Releases",
    credit:
      "Built on CC Connect, preserving upstream history and the MIT license.",
    skip: "Skip to content",
    eyebrow: "YOUR LOCAL AGENT. A CONVERSATION AWAY.",
    heroFirst: "Your agent stays local.",
    heroSecond: "Your ideas go anywhere.",
    heroBody:
      "Keep working with your coding agent from Feishu, Telegram, or Slack.\nYour code and runtime stay on your machine. Messages keep you connected.",
    copy: "Copy",
    start: "Installation guide",
    platformNote: "macOS · Linux · Windows · Open source",
    demoHeading: "From a message to the next step.",
    demoTabstart: "Start a task",
    demoTabsteer: "Add a direction",
    demoTabresult: "See the result",
    demoLabel: "Interactive example · No connection to your machine",
    userLabelstart: "You",
    requeststart:
      "Check the form error on the sign-in page. Find the cause first.",
    agentLabelstart: "Your local agent",
    startResponse:
      "Got it. I’ll inspect the form code, validation, and error messages.",
    startStatus: "In progress · Running in your local workspace",
    userLabelsteer: "You",
    requeststeer:
      "Check the form error on the sign-in page. Find the cause first.",
    directionsteer:
      "One more thing: keep the layout and only fix input validation.",
    agentLabelsteer: "Your local agent",
    steerResponse:
      "Direction added to the current task. I’ll keep the layout and adjust validation only.",
    steerStatus: "Example: native Codex steer · Other backends may queue",
    userLabelresult: "You",
    requestresult:
      "Check the form error on the sign-in page. Find the cause first.",
    directionresult:
      "One more thing: keep the layout and only fix input validation.",
    agentLabelresult: "Your local agent",
    resultResponse:
      "Fixed: empty input shows an error; valid input continues. The original layout is preserved.",
    resultStatus:
      "Example result · Ask a follow-up, review the diff, or plan the next step",
    demoFoot:
      "An illustration of message flow. Capabilities depend on your agent, platform, and configuration.",
    howEyebrow: "FAMILIAR TOOLS. A NEW CONNECTION.",
    howTitle: "From your desk to wherever you are.",
    howBody:
      "One Go binary connects your chat platforms and local coding agents.",
    stepTitle01: "Run your agent locally",
    stepBody01:
      "Use an installed, authenticated CLI such as Codex or Claude Code in your own workspace.",
    stepTitle02: "Bring the conversation along",
    stepBody02:
      "Connect Feishu, Telegram, Slack, and more. Send requests, receive progress, and keep collaborating.",
    stepTitle03: "Stay in control",
    stepBody03:
      "Configure workspaces and access per project. Permissions and capabilities depend on each integration.",
    feishuEyebrow: "BUILT WITH FEISHU IN MIND",
    feishuTitle: "A message opens up\na whole workflow.",
    feishuBody:
      "Streaming replies, permission prompts, and topic isolation keep longer tasks clear and manageable.",
    feishuLink: "Read the Feishu guide",
    cardsTitle: "One reply, continuously updated",
    cardsBody:
      "Follow progress in interactive cards and handle supported actions inside the conversation.",
    topicsTitle: "Keep topics focused",
    topicsBody:
      "Real topics can have separate sessions and workspace bindings to keep context organized.",
    steerTitle: "Add direction while work is running",
    steerBody:
      "Native Codex steer can update the active turn. Other agents fall back to queues according to their capabilities.",
    integrationTitle: "Use the combination you know.",
    integrationBody:
      "Choose your agent and your chat platform. See the docs for the full support matrix and limitations.",
    agentsLabel: "Local coding agents",
    platformsLabel: "Chat platforms",
    allIntegrations: "Explore all integrations and capabilities",
    startTitle: "Make your first connection.",
    startBody:
      "Start fresh, connect Feishu, or migrate from official CC Connect.",
    pathTitleinstall: "Start fresh",
    pathBodyinstall:
      "Give your agent the complete setup prompt, then verify a real reply and background operation.",
    pathTitlefeishu: "Connect Feishu",
    pathBodyfeishu:
      "Configure and authorize your bot; set an existing project’s work directory in the config file.",
    pathTitlemigration: "Migrate from CC Connect",
    pathBodymigration:
      "Inspect the migration plan with a dry run before choosing whether to switch services.",
    faqTitle: "A few common questions.",
    faqQlocal: "Does this website connect to my computer?",
    faqAlocal:
      "No. This website provides information, interactive examples, and setup instructions. Connections are handled by CC Connect Next installed and configured on your own machine.",
    faqQpublic: "Do I need a public IP address?",
    faqApublic:
      "Feishu uses a WebSocket connection and does not require a public IP, domain, or certificate. Other platforms have their own setup requirements.",
    faqQprivacy: "Where does my data go?",
    faqAprivacy:
      "Execution stays local. Messages pass through your chosen chat platform, and agent requests go to the configured provider. Local execution does not mean fully offline.",
    faqQcost: "Is CC Connect Next free?",
    faqAcost:
      "The code is MIT licensed. Agent subscriptions, model APIs, and chat platforms may have their own costs.",
    finalTitle: "Your next idea is\none message away.",
    finalCta: "Get connected",
    agentInstallEyebrow: "LET YOUR AGENT SET IT UP",
    agentInstallTitle: "From installation to your first reply.",
    agentInstallBody: "Copy the complete prompt into Codex, Claude Code, or another local coding agent. It handles configuration and verification; you complete sign-in, QR scanning, and required permissions.",
    agentInstallCopy: "Copy setup prompt",
    agentInstallDetails: "Read the complete prompt",
    installPrompt: "Install the published stable cc-connect-next from https://github.com/timmyagentic/cc-connect-next\nfor a first-time Feishu/Lark setup. Verify OS/architecture, the agent CLI and its login state,\nand the absolute project directory; prefer my current agent/project and ask only for missing\ninformation. Preserve existing CC Connect/Next configurations and services; do not migrate or overwrite.\nChoose an explicit absolute config path. If it does not exist, run `cc-connect-next --config <config-path>`\nto create it, then fill in the real work_dir, agent type, and its options yourself. Keep one project name.\nRun `cc-connect-next feishu setup --config <config-path> --project <project-name>`, show the QR code,\nand keep the process waiting while I scan it. Accept the recommended settings and confirm the official\nlark-cli companion; use `--recommended --lark-cli` for non-interactive execution, preserve existing\nprofiles, and make the same bot the default bot profile.\nRun `cc-connect-next doctor --config <config-path>`, then start in the foreground with\n`cc-connect-next --config <config-path>`. Ask me to send /version, /whoami, and an ordinary prompt\nin a private chat; verify replies and configure access\nusing the actual user ID. Stop the foreground process you started, then run\n`cc-connect-next daemon install --config <config-path>`. Check service status, Runtime, and platform readiness in\n`cc-connect-next daemon status` and verify replies still work in the background. Report version,\nconfig path, work_dir, and verification results. Call on me for login, QR scanning, or app publication;\nmark any incomplete real-message verification UNVERIFIED.",
    manualTitle: "Prefer to set up manually?",
    prepareTitle: "Prepare a working agent",
    prepareBody: "Install and sign in to a supported agent CLI, check that it can answer locally, and choose an existing project directory. Keep its absolute path ready. If you do not have an agent yet, complete its installation and sign-in first.",
    verifyTitle: "Start in the foreground and get a reply",
    verifyBody: "doctor checks the environment without connecting to a platform. After it passes, start in the foreground. Send /version, /whoami, and an ordinary question in a private chat, confirm the agent answers, and set access using the actual user ID.",
    backgroundTitle: "Move to the background and verify again",
    backgroundBody: "Stop the foreground process you started before installing the background service. Check the service, Runtime, and platform readiness, then send another ordinary question to confirm replies still work after the terminal is closed.",
    migrationEyebrow: "ALREADY USING OFFICIAL CC CONNECT?",
    docsEyebrow: "GET STARTED",
    docsTitle: "Connect your local agent.",
    docsIntro:
      "Give the complete setup prompt to an agent with local terminal access, or follow the five manual steps below. Check and preserve existing configurations and services first.",
    docsHeadinginstall: "Install and fill in the configuration",
    docsBodyinstall:
      "These examples use the default config path on macOS / Linux; use the corresponding absolute path on Windows. The first run creates a template and exits. Fill in the real work_dir, agent type, and project name, replace every REPLACE placeholder, and use this same config file throughout.",
    docsCopyinstall: "Copy commands",
    docsReadinstall: "Read the complete first-install guide",
    docsHeadingfeishu: "Connect Feishu",
    docsBodyfeishu:
      "Use the same project name from your config for bot setup and QR authorization, with the optional official lark-cli companion. feishu setup configures the bot; it does not fill in an existing project’s work_dir.",
    docsCopyfeishu: "Copy commands",
    docsReadfeishu: "Read the full guide",
    docsHeadingmigration: "Inspect a migration before switching",
    docsBodymigration:
      "The dry run only reports the plan. migrate --switch changes service running and autostart state; read the backup, compatibility, and rollback instructions first.",
    docsCopymigration: "Copy commands",
    docsReadmigration: "Read the full guide",
    nextTitle: "Keep exploring",
    usage: "Chat commands and daily use",
    config: "Configuration reference",
    manifest: "Agent capabilities and permissions",
    issues: "Report an issue",
    metaHome: "CC Connect Next — Your local agent, anywhere",
    metaDocs: "Get started — CC Connect Next",
    meta404: "Page not found — CC Connect Next",
    descriptionHome:
      "Work with local Codex, Claude Code, and other coding agents through Feishu, Telegram, Slack, and more.",
    descriptionDocs:
      "Use the complete setup prompt or five-step guide to prepare your agent, configure a project, connect Feishu, verify a reply, and run in the background.",
    description404: "The requested page could not be found.",
  },
};
export function resolveLanguage(value) {
  return typeof value === "string" && value.toLowerCase().startsWith("zh")
    ? "zh-CN"
    : "en";
}
let language = "zh-CN";
export function applyLanguage(value) {
  language = resolveLanguage(value);
  document.documentElement.lang = language;
  const page = document.body.dataset.page || "Home";
  document.title = translations[language]["meta" + page];
  document
    .querySelector('meta[property="og:title"]')
    ?.setAttribute("content", document.title);
  for (const selector of [
    'meta[name="description"]',
    'meta[property="og:description"]',
  ]) {
    document
      .querySelector(selector)
      ?.setAttribute("content", translations[language]["description" + page]);
  }
  document.querySelectorAll("[data-i18n]").forEach((node) => {
    const value = translations[language][node.dataset.i18n];
    if (value !== undefined) node.textContent = value;
  });
  document.querySelectorAll("[data-guide]").forEach((link) => {
    const suffix = language === "en" ? "" : ".zh-CN";
    link.href = `https://github.com/timmyagentic/cc-connect-next/blob/main/docs/${link.dataset.guide}${suffix}.md`;
  });
  document
    .querySelectorAll("[data-language]")
    .forEach((button) =>
      button.setAttribute(
        "aria-pressed",
        String(button.dataset.language === language),
      ),
    );
  return language;
}
function initialize() {
  let saved;
  try {
    saved = localStorage.getItem("cc-connect-next-language");
  } catch {}
  applyLanguage(saved || navigator.language);
  document.querySelectorAll("[data-language]").forEach((button) =>
    button.addEventListener("click", () => {
      applyLanguage(button.dataset.language);
      try {
        localStorage.setItem("cc-connect-next-language", language);
      } catch {}
    }),
  );
  const menu = document.querySelector(".menu-toggle");
  const nav = document.getElementById("navigation");
  const close = () => {
    menu.setAttribute("aria-expanded", "false");
    nav.classList.remove("is-open");
  };
  menu.addEventListener("click", () => {
    const open = menu.getAttribute("aria-expanded") !== "true";
    menu.setAttribute("aria-expanded", String(open));
    nav.classList.toggle("is-open", open);
  });
  nav.querySelectorAll("a").forEach((a) => a.addEventListener("click", close));
  document.addEventListener("keydown", (event) => {
    if (
      event.key === "Escape" &&
      menu.getAttribute("aria-expanded") === "true"
    ) {
      close();
      menu.focus();
    }
  });
  const tabs = [...document.querySelectorAll("[data-demo]")];
  const select = (tab, focus = false) => {
    tabs.forEach((item) => {
      item.setAttribute("aria-selected", String(item === tab));
      item.tabIndex = item === tab ? 0 : -1;
      document.getElementById(item.getAttribute("aria-controls")).hidden =
        item !== tab;
    });
    if (focus) tab.focus();
  };
  tabs.forEach((tab, index) => {
    tab.addEventListener("click", () => select(tab));
    tab.addEventListener("keydown", (event) => {
      let next;
      if (event.key === "ArrowRight") next = (index + 1) % tabs.length;
      if (event.key === "ArrowLeft")
        next = (index + tabs.length - 1) % tabs.length;
      if (event.key === "Home") next = 0;
      if (event.key === "End") next = tabs.length - 1;
      if (next !== undefined) {
        event.preventDefault();
        select(tabs[next], true);
      }
    });
  });
  document.querySelectorAll("[data-copy]").forEach((button) =>
    button.addEventListener("click", async () => {
      const target = document.getElementById(button.dataset.copy);
      const label =
        button.querySelector("[data-copy-label] [data-i18n]") ||
        button.querySelector("[data-copy-label]");
      const status = document.getElementById("copy-status");
      try {
        await navigator.clipboard.writeText(target.textContent);
        label.textContent = language === "en" ? "Copied" : "已复制";
        status.textContent =
          language === "en" ? "Copied to clipboard" : "已复制到剪贴板";
        setTimeout(() => applyLanguage(language), 1800);
      } catch {
        const details = target.closest("details");
        if (details) details.open = true;
        const range = document.createRange();
        range.selectNodeContents(target);
        const selection = window.getSelection();
        selection.removeAllRanges();
        selection.addRange(range);
        label.textContent =
          language === "en" ? "Select and copy" : "请手动复制";
        status.textContent =
          language === "en"
            ? "Clipboard unavailable. The content is selected for manual copy."
            : "剪贴板不可用，已选中内容，请手动复制。";
      }
    }),
  );
}
if (typeof document !== "undefined") initialize();
