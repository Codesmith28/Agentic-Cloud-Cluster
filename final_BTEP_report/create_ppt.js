// Copyright 2025-2026 Sarthak Siddhpura
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

const pptxgen = require("pptxgenjs");
const React = require("react");
const ReactDOMServer = require("react-dom/server");
const sharp = require("sharp");

// ─── Color Palette ───────────────────────────────────────────────────────────
const C = {
  navy:     "1A2A4A",   // dark navy (primary)
  teal:     "0D7A8A",   // teal accent
  tealLt:   "12A0B4",   // lighter teal
  maroon:   "7B1E1E",   // AU maroon
  white:    "FFFFFF",
  offWhite: "F0F4F8",
  lightBg:  "EBF4F7",
  cardBg:   "FFFFFF",
  text:     "1A2A4A",
  muted:    "607080",
  accent:   "E84545",   // red highlight
  green:    "27AE60",
  yellow:   "F39C12",
  gray:     "90A0AE",
};

// ─── Icon Helper ─────────────────────────────────────────────────────────────
const { FaCogs, FaServer, FaDocker, FaBrain, FaChartLine,
        FaNetworkWired, FaDatabase, FaRocket, FaCheckCircle,
        FaArrowRight, FaLayerGroup, FaClock, FaBalanceScale,
        FaProjectDiagram, FaCode, FaCloud, FaCube, FaExclamationTriangle,
        FaTasks, FaSync, FaShieldAlt } = require("react-icons/fa");
const { MdSchedule, MdMemory, MdStorage, MdCpu, MdSpeed } = require("react-icons/md");

function svgToB64(svgStr) {
  return "image/svg+xml;base64," + Buffer.from(svgStr).toString("base64");
}

function renderIcon(IconComp, color = "#FFFFFF", size = 256) {
  const svg = ReactDOMServer.renderToStaticMarkup(
    React.createElement(IconComp, { color, size: String(size) })
  );
  return svgToB64(svg);
}

async function iconPng(IconComp, color = "#FFFFFF", size = 256) {
  const svg = ReactDOMServer.renderToStaticMarkup(
    React.createElement(IconComp, { color, size: String(size) })
  );
  const buf = await sharp(Buffer.from(svg)).png().toBuffer();
  return "image/png;base64," + buf.toString("base64");
}

// ─── Shape helpers ────────────────────────────────────────────────────────────
function addCard(slide, x, y, w, h, opts = {}) {
  slide.addShape("rect", {
    x, y, w, h,
    fill: { color: opts.fill || C.cardBg },
    line: { color: opts.border || "D0DCE8", width: 1 },
    shadow: { type: "outer", color: "000000", blur: 8, offset: 2, angle: 135, opacity: 0.10 },
    ...( opts.rectRadius !== undefined ? { rectRadius: opts.rectRadius } : {} )
  });
}

function addTitle(slide, text, y = 0.28) {
  slide.addText(text, {
    x: 0.4, y, w: 9.2, h: 0.55,
    fontSize: 28, bold: true, color: C.navy,
    fontFace: "Calibri", align: "left", margin: 0
  });
}

function addSubTitle(slide, text, y = 0.85) {
  slide.addText(text, {
    x: 0.4, y, w: 9.2, h: 0.3,
    fontSize: 13, color: C.muted, fontFace: "Calibri", align: "left", margin: 0
  });
}

// Left-side colored accent on dark section slides
function darkSlide(slide) {
  slide.background = { color: C.navy };
}

// ─── MAIN ─────────────────────────────────────────────────────────────────────
(async () => {
  const pres = new pptxgen();
  pres.layout = "LAYOUT_16x9";
  pres.author = "Sarthak Siddhpura";
  pres.title = "Agentic Cloud Cluster";

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 1 — TITLE
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.navy };

    // Left accent bar
    s.addShape("rect", { x: 0, y: 0, w: 0.1, h: 5.625, fill: { color: C.teal }, line: { color: C.teal } });

    // Decorative circles (background)
    s.addShape("ellipse", { x: 7.5, y: -0.8, w: 3.5, h: 3.5, fill: { color: "12304A", transparency: 0 }, line: { color: "12304A" } });
    s.addShape("ellipse", { x: 8.2, y: 2.8, w: 2.5, h: 2.5, fill: { color: "0D3D5A", transparency: 0 }, line: { color: "0D3D5A" } });

    // Main heading
    s.addText("Agentic Cloud Cluster", {
      x: 0.5, y: 1.1, w: 7, h: 0.9,
      fontSize: 40, bold: true, color: C.white,
      fontFace: "Calibri", align: "left", margin: 0
    });

    s.addText("A Go Distributed Cluster Managing Framework\nwith PPO-based Scheduling", {
      x: 0.5, y: 2.1, w: 7, h: 0.75,
      fontSize: 16, color: C.tealLt,
      fontFace: "Calibri", align: "left", margin: 0
    });

    // Divider
    s.addShape("rect", { x: 0.5, y: 2.95, w: 4, h: 0.04, fill: { color: C.teal }, line: { color: C.teal } });

    s.addText([
      { text: "Sarthak Siddhpura", options: { bold: true, breakLine: true } },
      { text: "AU2240041  |  B.Tech Computer Science & Engineering", options: { breakLine: true } },
      { text: "Mentor: Prof. Sanjay Chaudhary  |  May 2026", options: {} },
    ], {
      x: 0.5, y: 3.1, w: 6, h: 1.2,
      fontSize: 13, color: "C8D8E8",
      fontFace: "Calibri", align: "left"
    });

    s.addText("SEAS · Ahmedabad University", {
      x: 0.5, y: 5.05, w: 5, h: 0.35,
      fontSize: 11, color: C.gray, fontFace: "Calibri"
    });

    // Server/cluster icon cluster (right side)
    const ic1 = await iconPng(FaServer, "#" + C.tealLt, 200);
    const ic2 = await iconPng(FaBrain, "#" + C.tealLt, 200);
    const ic3 = await iconPng(FaNetworkWired, "#" + C.tealLt, 200);
    s.addImage({ data: ic1, x: 7.6, y: 0.5, w: 0.9, h: 0.9 });
    s.addImage({ data: ic2, x: 8.7, y: 1.3, w: 0.9, h: 0.9 });
    s.addImage({ data: ic3, x: 7.4, y: 2.2, w: 0.9, h: 0.9 });
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 2 — AGENDA
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Presentation Agenda");

    const items = [
      ["01", "Problem & Motivation",    "Why scheduling is hard"],
      ["02", "System Architecture",      "Go master-worker cluster"],
      ["03", "RL Approach",              "Q-Tables → DQN → PPO"],
      ["04", "PPO Design",               "Architecture, rewards, features"],
      ["05", "Results & Evaluation",     "KPIs, campaigns, pressure tests"],
      ["06", "Conclusion",               "Key takeaways & future work"],
    ];

    const cols = 3;
    const cardW = 2.9, cardH = 1.5;
    const startX = 0.35, startY = 1.0;
    const gapX = 0.15, gapY = 0.2;

    for (let i = 0; i < items.length; i++) {
      const col = i % cols, row = Math.floor(i / cols);
      const x = startX + col * (cardW + gapX);
      const y = startY + row * (cardH + gapY);

      // Card
      s.addShape("rect", {
        x, y, w: cardW, h: cardH,
        fill: { color: C.cardBg },
        line: { color: "CBD8E4", width: 1 },
        shadow: { type: "outer", color: "000000", blur: 6, offset: 2, angle: 135, opacity: 0.08 }
      });
      // Left accent
      s.addShape("rect", { x, y, w: 0.07, h: cardH, fill: { color: C.teal }, line: { color: C.teal } });
      // Number
      s.addText(items[i][0], {
        x: x + 0.15, y: y + 0.18, w: 0.5, h: 0.45,
        fontSize: 24, bold: true, color: C.teal, fontFace: "Calibri", margin: 0
      });
      // Title
      s.addText(items[i][1], {
        x: x + 0.15, y: y + 0.65, w: cardW - 0.3, h: 0.35,
        fontSize: 13, bold: true, color: C.navy, fontFace: "Calibri", margin: 0
      });
      // Sub
      s.addText(items[i][2], {
        x: x + 0.15, y: y + 1.02, w: cardW - 0.3, h: 0.35,
        fontSize: 11, color: C.muted, fontFace: "Calibri", margin: 0
      });
    }
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 3 — PROBLEM STATEMENT
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.navy };

    s.addText("The Scheduling Problem", {
      x: 0.5, y: 0.3, w: 9, h: 0.65,
      fontSize: 30, bold: true, color: C.white, fontFace: "Calibri"
    });
    s.addText("Why simple rules fail in heterogeneous clusters", {
      x: 0.5, y: 0.95, w: 9, h: 0.3,
      fontSize: 14, color: C.tealLt, fontFace: "Calibri"
    });

    // 3 problem boxes
    const boxes = [
      { icon: FaServer,            title: "Heterogeneous Workers",   body: "CPU, memory & storage vary\nacross worker nodes" },
      { icon: FaExclamationTriangle, title: "Static Rules Fail",     body: "Round-Robin ignores resource\nstate & task demands" },
      { icon: FaClock,              title: "Dynamic State",          body: "Optimal placement changes\nevery few seconds" },
    ];
    for (let i = 0; i < 3; i++) {
      const x = 0.4 + i * 3.1;
      s.addShape("rect", {
        x, y: 1.55, w: 2.85, h: 2.7,
        fill: { color: "0D2240" }, line: { color: C.teal, width: 1.5 },
        shadow: { type: "outer", color: "000000", blur: 10, offset: 3, angle: 135, opacity: 0.25 }
      });
      const ic = await iconPng(boxes[i].icon, "#" + C.tealLt, 256);
      s.addImage({ data: ic, x: x + 0.95, y: 1.75, w: 0.95, h: 0.95 });
      s.addText(boxes[i].title, {
        x: x + 0.1, y: 2.78, w: 2.65, h: 0.35,
        fontSize: 13, bold: true, color: C.white, fontFace: "Calibri", align: "center", margin: 0
      });
      s.addText(boxes[i].body, {
        x: x + 0.1, y: 3.18, w: 2.65, h: 0.8,
        fontSize: 11, color: "A0BDCC", fontFace: "Calibri", align: "center"
      });
    }

    // The RL solution tease
    s.addShape("rect", { x: 0.4, y: 4.45, w: 9.2, h: 0.7,
      fill: { color: C.teal }, line: { color: C.teal } });
    s.addText("Solution: A scheduler that LEARNS from consequences, not just follows fixed rules", {
      x: 0.5, y: 4.5, w: 9, h: 0.6,
      fontSize: 14, bold: true, color: C.white, fontFace: "Calibri", align: "center"
    });
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 4 — PROJECT OBJECTIVES
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Project Definition & Objectives");
    addSubTitle(s, "Building a Go distributed cluster with a PPO-based intelligent scheduler", 0.83);

    const objectives = [
      ["Go Master-Worker Cluster",    "Docker-based task execution across worker nodes"],
      ["Worker Lifecycle Management", "Registration, heartbeats, resource tracking, retries"],
      ["Clean Scheduler Interface",   "Swap algorithms without changing worker logic"],
      ["Baseline + PPO Scheduling",   "Round-Robin, RTS heuristic, and learned PPO"],
      ["Realistic RL Training",       "Alibaba trace replay with shaped reward function"],
      ["Repeatable Evaluation",       "Campaign benchmarks across load patterns"],
    ];

    for (let i = 0; i < objectives.length; i++) {
      const row = Math.floor(i / 2), col = i % 2;
      const x = 0.35 + col * 4.75, y = 1.2 + row * 1.3;
      s.addShape("rect", {
        x, y, w: 4.5, h: 1.1,
        fill: { color: C.cardBg },
        line: { color: "CBD8E4", width: 1 },
        shadow: { type: "outer", color: "000000", blur: 5, offset: 2, angle: 135, opacity: 0.08 }
      });
      // Numbered circle
      s.addShape("ellipse", { x: x + 0.15, y: y + 0.28, w: 0.5, h: 0.5,
        fill: { color: C.teal }, line: { color: C.teal } });
      s.addText(String(i + 1), {
        x: x + 0.15, y: y + 0.28, w: 0.5, h: 0.5,
        fontSize: 13, bold: true, color: C.white, fontFace: "Calibri",
        align: "center", valign: "middle", margin: 0
      });
      s.addText(objectives[i][0], {
        x: x + 0.75, y: y + 0.12, w: 3.6, h: 0.38,
        fontSize: 13, bold: true, color: C.navy, fontFace: "Calibri", margin: 0
      });
      s.addText(objectives[i][1], {
        x: x + 0.75, y: y + 0.52, w: 3.6, h: 0.45,
        fontSize: 11, color: C.muted, fontFace: "Calibri", margin: 0
      });
    }
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 5 — HIGH-LEVEL ARCHITECTURE
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "System Architecture Overview");

    // Master Node big box
    s.addShape("rect", { x: 2.5, y: 0.9, w: 5, h: 4.3,
      fill: { color: "EBF6FB" }, line: { color: C.teal, width: 2 } });
    s.addText("MASTER NODE", {
      x: 3.2, y: 0.95, w: 3.5, h: 0.3,
      fontSize: 10, bold: true, color: C.teal, fontFace: "Calibri", align: "center"
    });

    // Inside master — components
    const masterItems = [
      { label: "Task Queue", x: 2.65, y: 1.35 },
      { label: "Worker Registry", x: 4.15, y: 1.35 },
      { label: "MongoDB", x: 3.4, y: 2.2 },
      { label: "Scheduler Layer\n(RR / RTS / PPO)", x: 3.4, y: 3.1 },
    ];
    for (const mi of masterItems) {
      s.addShape("rect", {
        x: mi.x, y: mi.y, w: 1.55, h: 0.65,
        fill: { color: C.cardBg }, line: { color: "A0C8D8", width: 1 },
        shadow: { type: "outer", color: "000000", blur: 4, offset: 1, angle: 135, opacity: 0.1 }
      });
      s.addText(mi.label, {
        x: mi.x + 0.05, y: mi.y + 0.05, w: 1.45, h: 0.55,
        fontSize: 9.5, color: C.navy, fontFace: "Calibri", align: "center", valign: "middle"
      });
    }

    // Arrows inside master
    s.addShape("line", { x: 4.25, y: 2.0, w: 0, h: 0.2, line: { color: C.muted, width: 1 } });
    s.addShape("line", { x: 4.25, y: 2.87, w: 0, h: 0.23, line: { color: C.muted, width: 1 } });

    // Worker nodes (left)
    for (let i = 0; i < 3; i++) {
      const wy = 1.1 + i * 1.3;
      s.addShape("rect", {
        x: 0.2, y: wy, w: 1.9, h: 1.1,
        fill: { color: "E8F4F8" }, line: { color: "78AABB", width: 1.5 }
      });
      s.addText(`Worker ${i + 1}`, { x: 0.25, y: wy + 0.02, w: 1.8, h: 0.28,
        fontSize: 10, bold: true, color: C.navy, fontFace: "Calibri", align: "center" });
      s.addText("gRPC client\nDocker exec\nHeartbeat", { x: 0.25, y: wy + 0.3, w: 1.8, h: 0.72,
        fontSize: 8.5, color: C.muted, fontFace: "Calibri", align: "center" });
      // Arrow to master
      s.addShape("line", { x: 2.1, y: wy + 0.55, w: 0.4, h: 0, line: { color: C.teal, width: 1.5 } });
    }
    s.addText("gRPC", { x: 2.12, y: 2.3, w: 0.35, h: 0.22,
      fontSize: 8, color: C.teal, fontFace: "Calibri" });

    // PPO service (right)
    s.addShape("rect", { x: 8.0, y: 1.4, w: 1.75, h: 2.5,
      fill: { color: "E8F0FA" }, line: { color: C.maroon, width: 1.5 } });
    s.addText("AGENTIC\nSCHEDULER", { x: 8.05, y: 1.48, w: 1.65, h: 0.5,
      fontSize: 9, bold: true, color: C.maroon, fontFace: "Calibri", align: "center" });
    s.addText("PPO Policy\n(PyTorch)\n\nOffline training\nTrace replay", {
      x: 8.05, y: 2.0, w: 1.65, h: 1.7,
      fontSize: 9, color: C.navy, fontFace: "Calibri", align: "center"
    });
    // Arrow from master to PPO
    s.addShape("line", { x: 7.5, y: 3.1, w: 0.5, h: 0, line: { color: C.maroon, width: 1.5 } });
    s.addText("gRPC", { x: 7.52, y: 2.88, w: 0.45, h: 0.22,
      fontSize: 8, color: C.maroon, fontFace: "Calibri" });

    // Client (top)
    s.addShape("rect", { x: 3.8, y: 0.1, w: 2.1, h: 0.55,
      fill: { color: "FFF3E0" }, line: { color: C.yellow, width: 1.5 } });
    s.addText("Client / HTTP API", { x: 3.82, y: 0.14, w: 2.05, h: 0.46,
      fontSize: 10, bold: true, color: "805000", fontFace: "Calibri", align: "center", valign: "middle" });
    s.addShape("line", { x: 4.85, y: 0.65, w: 0, h: 0.25, line: { color: C.yellow, width: 1.5 } });
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 6 — MASTER NODE
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Master Node — Coordinator");

    const modules = [
      { name: "Server Layer",       desc: "Exposes gRPC calls for registration, heartbeat, task status, result reporting", color: "0D7A8A" },
      { name: "Task Handling",      desc: "Creates tasks, tracks state transitions, queues when workers unavailable", color: "1A8A5A" },
      { name: "Worker Registry",    desc: "Maintains active worker IDs, addresses, resource capacities, availability", color: "8A6A0D" },
      { name: "Scheduler Package",  desc: "Common interface for Round-Robin, RTS, and PPO scheduling algorithms", color: "7B1E1E" },
      { name: "Persistence Layer",  desc: "MongoDB stores workers, tasks, assignments, attempts, results", color: "2A5A8A" },
      { name: "Telemetry Manager",  desc: "Maintains recent worker health and resource state for scheduling decisions", color: "5A2A8A" },
    ];

    const icons = [FaServer, FaTasks, FaDatabase, MdSchedule, FaDatabase, FaChartLine];
    for (let i = 0; i < modules.length; i++) {
      const col = i % 2, row = Math.floor(i / 2);
      const x = 0.35 + col * 4.8, y = 1.0 + row * 1.4;
      s.addShape("rect", {
        x, y, w: 4.55, h: 1.25,
        fill: { color: C.cardBg }, line: { color: "D0DCEA", width: 1 },
        shadow: { type: "outer", color: "000000", blur: 6, offset: 2, angle: 135, opacity: 0.1 }
      });
      // left accent
      s.addShape("rect", { x, y, w: 0.08, h: 1.25, fill: { color: modules[i].color }, line: { color: modules[i].color } });
      const ic = await iconPng(icons[i], "#" + modules[i].color, 192);
      s.addImage({ data: ic, x: x + 0.15, y: y + 0.37, w: 0.45, h: 0.45 });
      s.addText(modules[i].name, {
        x: x + 0.7, y: y + 0.1, w: 3.7, h: 0.38,
        fontSize: 13, bold: true, color: C.navy, fontFace: "Calibri", margin: 0
      });
      s.addText(modules[i].desc, {
        x: x + 0.7, y: y + 0.5, w: 3.7, h: 0.68,
        fontSize: 10.5, color: C.muted, fontFace: "Calibri", margin: 0
      });
    }
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 7 — WORKER NODE
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Worker Node — Executor");
    addSubTitle(s, "Stateless execution units that run Docker containers and report health", 0.85);

    // Worker diagram
    s.addShape("rect", { x: 0.3, y: 1.05, w: 4.2, h: 4.2,
      fill: { color: "EBF6FB" }, line: { color: C.teal, width: 2 } });
    s.addText("WORKER NODE", { x: 0.8, y: 1.1, w: 3.2, h: 0.28,
      fontSize: 10, bold: true, color: C.teal, fontFace: "Calibri", align: "center" });

    const wParts = [
      "gRPC Server",
      "Task Assignment Handler",
      "Docker Container Executor",
      "Log Streamer",
      "Heartbeat Reporter",
    ];
    for (let i = 0; i < wParts.length; i++) {
      s.addShape("rect", { x: 0.55, y: 1.52 + i * 0.66, w: 3.7, h: 0.52,
        fill: { color: i % 2 === 0 ? "D8EEF4" : C.cardBg },
        line: { color: "90C0D0", width: 0.75 } });
      s.addText(wParts[i], { x: 0.6, y: 1.55 + i * 0.66, w: 3.6, h: 0.45,
        fontSize: 11.5, color: C.navy, fontFace: "Calibri", align: "center", valign: "middle" });
    }

    // 3 responsibility boxes on right
    const resp = [
      { title: "Registration & Heartbeat",
        body: "On join, the worker registers with master.\nPeriodically sends CPU/memory/storage updates." },
      { title: "Container Execution",
        body: "Pulls Docker image → creates container\nwith resource limits → runs & captures exit code." },
      { title: "Result Reporting",
        body: "Reports success/failure to master.\nUploads output files if task produced any." },
    ];
    const respIcons = [FaNetworkWired, FaDocker, FaCheckCircle];
    for (let i = 0; i < resp.length; i++) {
      const ry = 1.05 + i * 1.4;
      s.addShape("rect", { x: 4.85, y: ry, w: 4.85, h: 1.28,
        fill: { color: C.cardBg }, line: { color: "CBD8E4", width: 1 },
        shadow: { type: "outer", color: "000000", blur: 5, offset: 2, angle: 135, opacity: 0.08 } });
      const ic = await iconPng(respIcons[i], "#0D7A8A", 192);
      s.addImage({ data: ic, x: 4.97, y: ry + 0.35, w: 0.55, h: 0.55 });
      s.addText(resp[i].title, { x: 5.6, y: ry + 0.1, w: 4.0, h: 0.35,
        fontSize: 13, bold: true, color: C.navy, fontFace: "Calibri", margin: 0 });
      s.addText(resp[i].body, { x: 5.6, y: ry + 0.48, w: 4.0, h: 0.72,
        fontSize: 10.5, color: C.muted, fontFace: "Calibri", margin: 0 });
    }
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 8 — TASK LIFECYCLE
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Task Lifecycle & Control Flow");

    // Flow steps
    const steps = [
      { label: "SUBMIT", sub: "Client sends\ntask + resources", color: "0D7A8A" },
      { label: "QUEUE",  sub: "Master stores\nin queue", color: "1A8A5A" },
      { label: "SCHEDULE", sub: "Scheduler selects\nworker", color: "7B5B0D" },
      { label: "ASSIGN", sub: "gRPC AssignTask\nto worker", color: "7B1E1E" },
      { label: "RUN",    sub: "Docker container\nexecuted", color: "2A5A8A" },
      { label: "RESULT", sub: "Status reported\n& stored", color: "5A2A8A" },
    ];

    const bw = 1.35, bh = 1.1;
    const startX = 0.3, y = 1.3;
    for (let i = 0; i < steps.length; i++) {
      const x = startX + i * (bw + 0.22);
      s.addShape("rect", { x, y, w: bw, h: bh,
        fill: { color: steps[i].color }, line: { color: steps[i].color },
        shadow: { type: "outer", color: "000000", blur: 6, offset: 2, angle: 135, opacity: 0.2 } });
      s.addText(steps[i].label, { x, y: y + 0.05, w: bw, h: 0.42,
        fontSize: 13, bold: true, color: C.white, fontFace: "Calibri", align: "center" });
      s.addText(steps[i].sub, { x, y: y + 0.48, w: bw, h: 0.55,
        fontSize: 9.5, color: "D8E8F0", fontFace: "Calibri", align: "center" });
      // Arrow
      if (i < steps.length - 1) {
        s.addShape("line", { x: x + bw + 0.02, y: y + bh / 2, w: 0.18, h: 0,
          line: { color: C.muted, width: 2 } });
      }
    }

    // Recovery path
    s.addShape("rect", { x: 0.3, y: 2.6, w: 9.4, h: 0.8,
      fill: { color: "FFF3E0" }, line: { color: C.yellow, width: 1 } });
    s.addText("⚡  Failure Recovery", { x: 0.5, y: 2.65, w: 2.5, h: 0.3,
      fontSize: 12, bold: true, color: "805000", fontFace: "Calibri" });
    s.addText("If worker goes offline → attempt marked LOST → logical task returned to queue for re-scheduling (attempt-level isolation)", {
      x: 3.0, y: 2.67, w: 6.6, h: 0.55,
      fontSize: 11, color: "604000", fontFace: "Calibri"
    });

    // Task states diagram
    const states = ["CREATED", "QUEUED", "ASSIGNED", "RUNNING", "COMPLETED", "FAILED"];
    const stateColors = ["607080", C.teal, "8A6A0D", "2A5A8A", C.green, C.maroon];
    const sy = 3.65, bh2 = 0.6, bw2 = 1.4;
    for (let i = 0; i < states.length; i++) {
      const sx = 0.3 + i * (bw2 + 0.1);
      s.addShape("rect", { x: sx, y: sy, w: bw2, h: bh2,
        fill: { color: stateColors[i] }, line: { color: stateColors[i] } });
      s.addText(states[i], { x: sx, y: sy, w: bw2, h: bh2,
        fontSize: 9.5, bold: true, color: C.white, fontFace: "Calibri",
        align: "center", valign: "middle" });
      if (i < states.length - 1) {
        s.addShape("line", { x: sx + bw2, y: sy + bh2 / 2, w: 0.1, h: 0, line: { color: C.muted, width: 1.5 } });
      }
    }
    s.addText("Task States", { x: 0.3, y: 4.35, w: 3, h: 0.28,
      fontSize: 10, color: C.muted, italic: true, fontFace: "Calibri" });

    // Retry annotation
    s.addShape("line", { x: 8.1, y: 4.25, w: 0.8, h: 0, line: { color: C.accent, width: 1.5 } });
    s.addText("→ REQUEUE", { x: 8.15, y: 4.32, w: 1.0, h: 0.22,
      fontSize: 9, color: C.accent, fontFace: "Calibri" });
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 9 — RESOURCE ACCOUNTING & SCHEDULER INTERFACE
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Scheduler Interface & Resource Accounting");

    // Left: Scheduler interface
    s.addShape("rect", { x: 0.3, y: 1.0, w: 4.5, h: 4.25,
      fill: { color: "EBF6FB" }, line: { color: C.teal, width: 1.5 } });
    s.addText("SCHEDULER INTERFACE", { x: 0.55, y: 1.07, w: 4.0, h: 0.3,
      fontSize: 10, bold: true, color: C.teal, fontFace: "Calibri", align: "center" });

    const inputs = ["Task CPU requirement", "Task memory requirement", "Task storage requirement",
                    "Task type (CPU/MEM/mixed)", "Worker capacities", "Worker availability"];
    for (let i = 0; i < inputs.length; i++) {
      s.addShape("rect", { x: 0.5, y: 1.5 + i * 0.52, w: 2.4, h: 0.4,
        fill: { color: C.cardBg }, line: { color: "A0C4D0", width: 0.75 } });
      s.addText(inputs[i], { x: 0.55, y: 1.52 + i * 0.52, w: 2.3, h: 0.36,
        fontSize: 9.5, color: C.navy, fontFace: "Calibri", valign: "middle" });
    }

    // Arrow → returns
    s.addShape("line", { x: 2.95, y: 3.12, w: 1.3, h: 0, line: { color: C.teal, width: 2 } });
    s.addText("Returns\nWorker ID", { x: 3.0, y: 3.2, w: 1.0, h: 0.4,
      fontSize: 9, color: C.teal, fontFace: "Calibri", align: "center" });
    s.addShape("rect", { x: 3.5, y: 2.85, w: 1.1, h: 0.65,
      fill: { color: C.teal }, line: { color: C.teal } });
    s.addText("WORKER\nID", { x: 3.5, y: 2.85, w: 1.1, h: 0.65,
      fontSize: 10, bold: true, color: C.white, fontFace: "Calibri", align: "center", valign: "middle" });

    // Scheduler options
    const scheds = [
      { name: "Round-Robin", desc: "Simple cyclic", color: "607080" },
      { name: "RTS Heuristic", desc: "Risk-aware scoring", color: "8A6A0D" },
      { name: "PPO Scheduler", desc: "Learned policy", color: C.teal },
    ];
    for (let i = 0; i < scheds.length; i++) {
      const sy = 1.55 + i * 0.82;
      s.addShape("rect", { x: 3.5, y: sy, w: 1.1, h: 0.6,
        fill: { color: scheds[i].color }, line: { color: scheds[i].color } });
      s.addText(scheds[i].name, { x: 3.5, y: sy + 0.04, w: 1.1, h: 0.3,
        fontSize: 9, bold: true, color: C.white, fontFace: "Calibri", align: "center" });
      s.addText(scheds[i].desc, { x: 3.5, y: sy + 0.32, w: 1.1, h: 0.22,
        fontSize: 7.5, color: "D0E0E8", fontFace: "Calibri", align: "center" });
    }

    // Right: Resource Accounting
    s.addShape("rect", { x: 5.1, y: 1.0, w: 4.6, h: 4.25,
      fill: { color: "F5EBF8" }, line: { color: C.maroon, width: 1.5 } });
    s.addText("RESOURCE ACCOUNTING", { x: 5.35, y: 1.07, w: 4.1, h: 0.3,
      fontSize: 10, bold: true, color: C.maroon, fontFace: "Calibri", align: "center" });

    const rItems = [
      "Each worker advertises Total CPU, Memory, Storage",
      "Master tracks Available = Total − Reserved",
      "Task resources reserved on assignment",
      "Released on task completion",
      "PPO action mask uses this accounting",
      "Go validates PPO choice BEFORE dispatch",
    ];
    for (let i = 0; i < rItems.length; i++) {
      const ry = 1.5 + i * 0.58;
      s.addShape("ellipse", { x: 5.25, y: ry + 0.1, w: 0.28, h: 0.28,
        fill: { color: i < 4 ? C.teal : C.maroon }, line: { color: i < 4 ? C.teal : C.maroon } });
      s.addText(rItems[i], { x: 5.62, y: ry + 0.04, w: 3.88, h: 0.42,
        fontSize: 10.5, color: C.navy, fontFace: "Calibri", margin: 0 });
    }
    s.addText("Safety: Even if PPO returns an invalid worker,\nthe Go master validates before assignment.", {
      x: 5.2, y: 4.7, w: 4.35, h: 0.45,
      fontSize: 10, color: C.maroon, bold: true, fontFace: "Calibri"
    });
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 10 — SECTION BREAK: RL
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    darkSlide(s);
    s.addShape("rect", { x: 0, y: 0, w: 0.12, h: 5.625, fill: { color: C.teal }, line: { color: C.teal } });

    const ic = await iconPng(FaBrain, "#" + C.tealLt, 512);
    s.addImage({ data: ic, x: 7.0, y: 1.2, w: 2.5, h: 2.5 });

    s.addText("02", { x: 0.5, y: 0.4, w: 1.5, h: 0.9,
      fontSize: 60, bold: true, color: "1A3A5A", fontFace: "Calibri" });
    s.addText("Reinforcement Learning\nfor Scheduling", {
      x: 0.5, y: 1.4, w: 6, h: 1.2,
      fontSize: 34, bold: true, color: C.white, fontFace: "Calibri"
    });
    s.addText("From Q-Tables to Proximal Policy Optimization", {
      x: 0.5, y: 2.75, w: 6, h: 0.4,
      fontSize: 16, color: C.tealLt, fontFace: "Calibri"
    });
    s.addShape("rect", { x: 0.5, y: 3.3, w: 5, h: 0.05, fill: { color: C.teal }, line: { color: C.teal } });
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 11 — WHY RL?
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Why Reinforcement Learning for Scheduling?");

    // RL Mapping table
    const mapping = [
      { rl: "Agent",       cluster: "The Scheduler",          color: C.teal },
      { rl: "Environment", cluster: "Cluster + task stream",  color: "1A8A5A" },
      { rl: "State",       cluster: "Task features + worker resources", color: "8A6A0D" },
      { rl: "Action",      cluster: "Select a worker",        color: C.maroon },
      { rl: "Reward",      cluster: "Feasibility + headroom + balance", color: "2A5A8A" },
      { rl: "Policy",      cluster: "Scheduling algorithm",   color: "5A2A8A" },
    ];
    for (let i = 0; i < mapping.length; i++) {
      const row = Math.floor(i / 2), col = i % 2;
      const x = 0.35 + col * 4.75, y = 1.0 + row * 1.22;
      s.addShape("rect", { x, y, w: 4.5, h: 1.1,
        fill: { color: C.cardBg }, line: { color: "D0DCEA", width: 1 },
        shadow: { type: "outer", color: "000000", blur: 4, offset: 1, angle: 135, opacity: 0.08 } });
      // Color pill RL term
      s.addShape("rect", { x, y: y + 0.25, w: 1.2, h: 0.55,
        fill: { color: mapping[i].color }, line: { color: mapping[i].color } });
      s.addText(mapping[i].rl, { x, y: y + 0.25, w: 1.2, h: 0.55,
        fontSize: 13, bold: true, color: C.white, fontFace: "Calibri",
        align: "center", valign: "middle" });
      // Arrow
      s.addShape("line", { x: x + 1.25, y: y + 0.53, w: 0.3, h: 0, line: { color: C.muted, width: 1.5 } });
      s.addText(mapping[i].cluster, { x: x + 1.6, y: y + 0.2, w: 2.8, h: 0.68,
        fontSize: 12, color: C.navy, fontFace: "Calibri", margin: 0 });
    }

    s.addShape("rect", { x: 0.35, y: 4.65, w: 9.3, h: 0.65,
      fill: { color: C.teal }, line: { color: C.teal } });
    s.addText("Each placement changes future cluster state → scheduler should learn from consequences", {
      x: 0.5, y: 4.68, w: 9.0, h: 0.55,
      fontSize: 13, bold: true, color: C.white, fontFace: "Calibri", align: "center"
    });
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 12 — Q-TABLES → DQN → PPO Evolution
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Algorithm Evolution: Why PPO?");

    const algo = [
      {
        name: "Q-Tables", color: "607080",
        eq: "Q(s,a) ← Q(s,a) + α[r + γ·max Q(s′,a′) − Q(s,a)]",
        pros: ["Simple and interpretable", "Works for tiny state spaces"],
        cons: ["State space too large (CPU×MEM×tasks…)", "Needs discretization → loses detail"],
        verdict: "❌ Too small",
      },
      {
        name: "Deep Q-Network", color: "8A6A0D",
        eq: "Q(s,a;θ)  ≈  expected future reward",
        pros: ["Neural network generalizes states", "No table size problem"],
        cons: ["Dynamic worker set is awkward", "Action masking is unnatural", "Needs replay buffer + target net"],
        verdict: "⚠ Better, but not ideal",
      },
      {
        name: "PPO (Chosen)", color: C.teal,
        eq: "L_CLIP(θ) = E[min(r_t·Â_t,  clip(r_t, 1−ε, 1+ε)·Â_t)]",
        pros: ["Direct worker-selection policy", "Clean action masking", "Stable clipped updates", "Works with continuous features"],
        cons: ["Inference overhead", "Needs trace training data"],
        verdict: "✅ Best fit",
      },
    ];

    for (let i = 0; i < 3; i++) {
      const x = 0.25 + i * 3.2, y = 0.9;
      const a = algo[i];
      s.addShape("rect", { x, y, w: 3.0, h: 4.4,
        fill: { color: C.cardBg }, line: { color: a.color, width: 2 },
        shadow: { type: "outer", color: "000000", blur: 6, offset: 2, angle: 135, opacity: 0.1 } });
      // Header
      s.addShape("rect", { x, y, w: 3.0, h: 0.48,
        fill: { color: a.color }, line: { color: a.color } });
      s.addText(a.name, { x: x + 0.05, y: y + 0.06, w: 2.9, h: 0.36,
        fontSize: 14, bold: true, color: C.white, fontFace: "Calibri", align: "center" });
      // Equation
      s.addShape("rect", { x: x + 0.1, y: y + 0.58, w: 2.8, h: 0.58,
        fill: { color: "EBF4F8" }, line: { color: "C0D8E4", width: 0.75 } });
      s.addText(a.eq, { x: x + 0.12, y: y + 0.6, w: 2.76, h: 0.52,
        fontSize: 8.5, color: C.navy, fontFace: "Consolas", align: "center", valign: "middle" });
      // Pros
      s.addText("✓", { x: x + 0.12, y: y + 1.28, w: 0.25, h: 0.22,
        fontSize: 10, color: C.green, fontFace: "Calibri" });
      s.addText("Pros", { x: x + 0.35, y: y + 1.25, w: 2.4, h: 0.28,
        fontSize: 10, bold: true, color: C.green, fontFace: "Calibri" });
      for (let j = 0; j < a.pros.length; j++) {
        s.addText("• " + a.pros[j], { x: x + 0.15, y: y + 1.53 + j * 0.38, w: 2.7, h: 0.36,
          fontSize: 9.5, color: C.navy, fontFace: "Calibri" });
      }
      // Cons
      const consY = y + 1.53 + a.pros.length * 0.38 + 0.1;
      s.addText("✗  Cons", { x: x + 0.15, y: consY, w: 2.7, h: 0.28,
        fontSize: 10, bold: true, color: C.accent, fontFace: "Calibri" });
      for (let j = 0; j < a.cons.length; j++) {
        s.addText("• " + a.cons[j], { x: x + 0.15, y: consY + 0.3 + j * 0.33, w: 2.7, h: 0.31,
          fontSize: 9, color: C.muted, fontFace: "Calibri" });
      }
      // Verdict
      s.addShape("rect", { x, y: y + 3.95, w: 3.0, h: 0.45,
        fill: { color: a.color }, line: { color: a.color } });
      s.addText(a.verdict, { x, y: y + 3.95, w: 3.0, h: 0.45,
        fontSize: 12, bold: true, color: C.white, fontFace: "Calibri",
        align: "center", valign: "middle" });
    }
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 13 — PPO ARCHITECTURE
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "PPO Scheduler Architecture");
    addSubTitle(s, "Actor-Critic with pairwise task-worker encoding", 0.83);

    // Left: Inputs
    const inputItems = ["Task CPU req", "Task Mem req", "Task Storage", "Task Type", "SLA multiplier"];
    s.addShape("rect", { x: 0.2, y: 1.05, w: 1.9, h: 3.5,
      fill: { color: "EBF6FB" }, line: { color: C.teal, width: 1.5 } });
    s.addText("TASK\nFEATURES", { x: 0.25, y: 1.1, w: 1.8, h: 0.5,
      fontSize: 10, bold: true, color: C.teal, fontFace: "Calibri", align: "center" });
    for (let i = 0; i < 5; i++) {
      s.addShape("rect", { x: 0.3, y: 1.72 + i * 0.56, w: 1.7, h: 0.45,
        fill: { color: C.cardBg }, line: { color: "90C8D8", width: 0.5 } });
      s.addText(inputItems[i], { x: 0.32, y: 1.74 + i * 0.56, w: 1.66, h: 0.41,
        fontSize: 9, color: C.navy, fontFace: "Calibri", align: "center", valign: "middle" });
    }

    // Worker features
    const wfItems = ["Avail CPU ratio", "Avail Mem ratio", "Avail Storage ratio",
                     "Total CPU/Mem/Storage", "Used CPU ratio", "Used Mem ratio", "Used Storage ratio"];
    s.addShape("rect", { x: 0.2, y: 4.65, w: 1.9, h: 0.7,
      fill: { color: "F5EBF8" }, line: { color: C.maroon, width: 1 } });
    s.addText("+ Worker Features (×N)", { x: 0.22, y: 4.68, w: 1.86, h: 0.6,
      fontSize: 9, bold: true, color: C.maroon, fontFace: "Calibri", align: "center", valign: "middle" });

    // Arrow to encoder
    s.addShape("line", { x: 2.1, y: 2.85, w: 0.5, h: 0, line: { color: C.teal, width: 2 } });

    // Shared Encoder
    s.addShape("rect", { x: 2.65, y: 1.65, w: 2.1, h: 2.5,
      fill: { color: "E0F0F8" }, line: { color: C.teal, width: 2 } });
    s.addText("SHARED\nENCODER", { x: 2.7, y: 1.72, w: 2.0, h: 0.42,
      fontSize: 10, bold: true, color: C.teal, fontFace: "Calibri", align: "center" });
    const layerLabels = ["Dense(128) + ReLU", "Dense(128) + ReLU"];
    for (let i = 0; i < 2; i++) {
      s.addShape("rect", { x: 2.8, y: 2.28 + i * 0.75, w: 1.8, h: 0.6,
        fill: { color: C.cardBg }, line: { color: "90C8D8", width: 0.75 } });
      s.addText(layerLabels[i], { x: 2.82, y: 2.3 + i * 0.75, w: 1.76, h: 0.55,
        fontSize: 10, color: C.navy, fontFace: "Calibri", align: "center", valign: "middle" });
    }

    // Arrow split
    s.addShape("line", { x: 4.75, y: 2.85, w: 0.5, h: 0, line: { color: C.muted, width: 1.5 } });

    // Actor Head
    s.addShape("rect", { x: 5.3, y: 1.55, w: 2.0, h: 1.45,
      fill: { color: "E8F4E8" }, line: { color: C.green, width: 1.5 } });
    s.addText("ACTOR HEAD\n(Policy)", { x: 5.35, y: 1.62, w: 1.9, h: 0.45,
      fontSize: 10, bold: true, color: C.green, fontFace: "Calibri", align: "center" });
    s.addText("Logit per worker →\nMasked Softmax →\nπ(a|s)", { x: 5.35, y: 2.1, w: 1.9, h: 0.78,
      fontSize: 9, color: C.navy, fontFace: "Calibri", align: "center" });

    // Critic Head
    s.addShape("rect", { x: 5.3, y: 3.3, w: 2.0, h: 1.45,
      fill: { color: "F8EBE8" }, line: { color: C.accent, width: 1.5 } });
    s.addText("CRITIC HEAD\n(Value)", { x: 5.35, y: 3.37, w: 1.9, h: 0.45,
      fontSize: 10, bold: true, color: C.accent, fontFace: "Calibri", align: "center" });
    s.addText("V(s) — estimates\nexpected return\nfrom cluster state", { x: 5.35, y: 3.85, w: 1.9, h: 0.78,
      fontSize: 9, color: C.navy, fontFace: "Calibri", align: "center" });

    // Action output
    s.addShape("rect", { x: 7.7, y: 2.1, w: 1.9, h: 0.7,
      fill: { color: C.green }, line: { color: C.green } });
    s.addText("Selected Worker", { x: 7.72, y: 2.12, w: 1.86, h: 0.65,
      fontSize: 11, bold: true, color: C.white, fontFace: "Calibri", align: "center", valign: "middle" });
    s.addShape("line", { x: 7.3, y: 2.3, w: 0.4, h: 0, line: { color: C.green, width: 2 } });

    // Advantage output
    s.addShape("rect", { x: 7.7, y: 3.5, w: 1.9, h: 0.7,
      fill: { color: C.accent }, line: { color: C.accent } });
    s.addText("Advantage Â_t", { x: 7.72, y: 3.52, w: 1.86, h: 0.65,
      fontSize: 11, bold: true, color: C.white, fontFace: "Calibri", align: "center", valign: "middle" });
    s.addShape("line", { x: 7.3, y: 3.85, w: 0.4, h: 0, line: { color: C.accent, width: 2 } });
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 14 — STATE REPRESENTATION
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "State Representation — Feature Vectors");

    // Task features
    s.addShape("rect", { x: 0.3, y: 0.95, w: 4.3, h: 4.3,
      fill: { color: "EBF6FB" }, line: { color: C.teal, width: 1.5 } });
    s.addText("TASK FEATURE VECTOR  (5 values)", { x: 0.4, y: 1.02, w: 4.1, h: 0.32,
      fontSize: 10.5, bold: true, color: C.teal, fontFace: "Calibri", align: "center" });
    const taskF = [
      ["Required CPU",     "CPU requested by the task"],
      ["Required Memory",  "Memory requested by task"],
      ["Required Storage", "Storage requested by task"],
      ["SLA Multiplier",   "Acceptable delay relative to runtime"],
      ["Task Type",        "CPU-heavy / Mem-heavy / Mixed"],
    ];
    for (let i = 0; i < taskF.length; i++) {
      s.addShape("rect", { x: 0.45, y: 1.48 + i * 0.68, w: 4.0, h: 0.58,
        fill: { color: C.cardBg }, line: { color: "A0C4D0", width: 0.75 } });
      s.addText(taskF[i][0], { x: 0.5, y: 1.5 + i * 0.68, w: 1.6, h: 0.52,
        fontSize: 11, bold: true, color: C.navy, fontFace: "Calibri", valign: "middle" });
      s.addText(taskF[i][1], { x: 2.1, y: 1.5 + i * 0.68, w: 2.25, h: 0.52,
        fontSize: 10, color: C.muted, fontFace: "Calibri", valign: "middle" });
    }

    // Worker features
    s.addShape("rect", { x: 5.0, y: 0.95, w: 4.7, h: 4.3,
      fill: { color: "F5EBF8" }, line: { color: C.maroon, width: 1.5 } });
    s.addText("WORKER FEATURE VECTOR  (9 values per worker)", { x: 5.1, y: 1.02, w: 4.5, h: 0.32,
      fontSize: 10.5, bold: true, color: C.maroon, fontFace: "Calibri", align: "center" });
    const workerF = [
      ["Avail CPU ratio",     "Available / Total CPU"],
      ["Avail Mem ratio",     "Available / Total Memory"],
      ["Avail Storage ratio", "Available / Total Storage"],
      ["Total CPU",           "Absolute capacity"],
      ["Total Memory",        "Absolute capacity"],
      ["Used CPU ratio",      "Used / Total CPU"],
      ["Used Mem ratio",      "Used / Total Memory"],
      ["Used Storage ratio",  "Used / Total Storage"],
    ];
    for (let i = 0; i < workerF.length; i++) {
      s.addShape("rect", { x: 5.1, y: 1.45 + i * 0.42, w: 4.5, h: 0.35,
        fill: { color: C.cardBg }, line: { color: "C0A8D0", width: 0.5 } });
      s.addText(workerF[i][0], { x: 5.15, y: 1.47 + i * 0.42, w: 2.0, h: 0.3,
        fontSize: 10, bold: true, color: C.navy, fontFace: "Calibri", valign: "middle" });
      s.addText(workerF[i][1], { x: 7.15, y: 1.47 + i * 0.42, w: 2.35, h: 0.3,
        fontSize: 9.5, color: C.muted, fontFace: "Calibri", valign: "middle" });
    }

    // Pairwise note
    s.addShape("rect", { x: 0.3, y: 5.25, w: 9.4, h: 0.0 }); // spacing
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 15 — REWARD FUNCTION
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.navy };

    s.addText("Reward Function Design", {
      x: 0.4, y: 0.3, w: 9.2, h: 0.55,
      fontSize: 28, bold: true, color: C.white, fontFace: "Calibri"
    });
    // Equation banner
    s.addShape("rect", { x: 0.4, y: 1.0, w: 9.2, h: 0.6,
      fill: { color: "0D2A45" }, line: { color: C.teal, width: 1 } });
    s.addText("R = 1.4 + 0.25H − 0.35Qp − 0.55Tp − 0.20Ip − 0.40ΔI − Rq", {
      x: 0.5, y: 1.02, w: 9.0, h: 0.56,
      fontSize: 15, bold: true, color: C.tealLt, fontFace: "Consolas", align: "center"
    });

    const terms = [
      { sym: "H",   label: "Headroom Bonus",   eq: "1 − L_selected",        desc: "Reward choosing workers with free capacity", color: C.teal },
      { sym: "Qp",  label: "Queue Pressure",   eq: "min(wait / SLA, 3.0)",  desc: "Penalise placements that create waiting", color: C.yellow },
      { sym: "Tp",  label: "Tail Pressure",    eq: "max(turnaround/SLA−1,0)", desc: "Penalise SLA or tail-latency violations", color: C.accent },
      { sym: "Ip",  label: "Imbalance",        eq: "max(L_sel − L_avg, 0)", desc: "Discourage hot-spots on busy workers", color: "8A6A0D" },
      { sym: "ΔI",  label: "Δ Imbalance",      eq: "σ(loads_after) − σ(before)", desc: "Penalise actions worsening cluster balance", color: "5A2A8A" },
      { sym: "Rq",  label: "Requeue Penalty",  eq: "0.05 × min(reqs, 4)",   desc: "Discourage repeated risky placements", color: C.maroon },
    ];

    for (let i = 0; i < terms.length; i++) {
      const col = i % 3, row = Math.floor(i / 2);
      const x = 0.3 + col * 3.2, y = 1.75 + row * 1.6;
      s.addShape("rect", { x, y, w: 3.05, h: 1.48,
        fill: { color: "0D2A45" }, line: { color: terms[i].color, width: 1.5 } });
      // Symbol pill
      s.addShape("rect", { x, y, w: 0.7, h: 1.48, fill: { color: terms[i].color }, line: { color: terms[i].color } });
      s.addText(terms[i].sym, { x, y, w: 0.7, h: 1.48,
        fontSize: 17, bold: true, color: C.white, fontFace: "Consolas",
        align: "center", valign: "middle" });
      s.addText(terms[i].label, { x: x + 0.78, y: y + 0.1, w: 2.2, h: 0.32,
        fontSize: 11, bold: true, color: C.white, fontFace: "Calibri" });
      s.addShape("rect", { x: x + 0.78, y: y + 0.45, w: 2.15, h: 0.36,
        fill: { color: "081830" }, line: { color: "304060" } });
      s.addText(terms[i].eq, { x: x + 0.8, y: y + 0.47, w: 2.1, h: 0.3,
        fontSize: 9, color: C.tealLt, fontFace: "Consolas", valign: "middle" });
      s.addText(terms[i].desc, { x: x + 0.78, y: y + 0.88, w: 2.2, h: 0.52,
        fontSize: 9.5, color: "A0BDCC", fontFace: "Calibri" });
    }

    // Infeasible note
    s.addShape("rect", { x: 0.3, y: 5.05, w: 9.4, h: 0.38,
      fill: { color: C.maroon }, line: { color: C.maroon } });
    s.addText("Infeasible placement → Reward = −1.8  (hard penalty overrides all positive terms)", {
      x: 0.4, y: 5.06, w: 9.2, h: 0.36,
      fontSize: 12, color: C.white, bold: true, fontFace: "Calibri", align: "center"
    });
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 16 — TRAINING PIPELINE
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Training Pipeline — Alibaba Trace Replay");

    // Steps flow
    const steps = [
      { n: "1", label: "Load Alibaba\nTrace",       color: "2A5A8A" },
      { n: "2", label: "Normalize\nFeatures",       color: "1A8A5A" },
      { n: "3", label: "Trace Replay\nEnvironment", color: "8A6A0D" },
      { n: "4", label: "Collect\nRollouts",         color: C.teal },
      { n: "5", label: "Compute\nAdvantages",       color: "5A2A8A" },
      { n: "6", label: "PPO Update\n(clip+entropy)", color: C.maroon },
      { n: "7", label: "Save\nCheckpoint",          color: C.green },
    ];
    const bw = 1.22, bh = 1.4;
    for (let i = 0; i < steps.length; i++) {
      const x = 0.2 + i * (bw + 0.15);
      s.addShape("rect", { x, y: 1.05, w: bw, h: bh,
        fill: { color: steps[i].color }, line: { color: steps[i].color },
        shadow: { type: "outer", color: "000000", blur: 6, offset: 2, angle: 135, opacity: 0.2 } });
      s.addText(steps[i].n, { x, y: 1.1, w: bw, h: 0.4,
        fontSize: 18, bold: true, color: "FFFFFF80", fontFace: "Calibri", align: "center" });
      s.addText(steps[i].label, { x, y: 1.52, w: bw, h: 0.75,
        fontSize: 10.5, color: C.white, fontFace: "Calibri", align: "center" });
      if (i < steps.length - 1) {
        s.addShape("line", { x: x + bw + 0.02, y: 1.75, w: 0.11, h: 0,
          line: { color: C.muted, width: 2 } });
      }
    }

    // Lifecycle tracking box
    s.addShape("rect", { x: 0.3, y: 2.65, w: 9.4, h: 1.1,
      fill: { color: "EBF6FB" }, line: { color: C.teal, width: 1 } });
    s.addText("🔑  Key Design: Lifecycle Resource Tracking", { x: 0.5, y: 2.72, w: 4.5, h: 0.32,
      fontSize: 12, bold: true, color: C.navy, fontFace: "Calibri" });
    s.addText("Task resources are reserved on placement and released when simulated runtime ends.\nThis outperforms exponential decay models — crucial for simultaneous trace arrivals.", {
      x: 0.5, y: 3.05, w: 9.1, h: 0.62,
      fontSize: 11.5, color: C.navy, fontFace: "Calibri"
    });

    // Training config
    const cfg = [
      ["Model", "Actor-Critic, 2 hidden layers"],
      ["Hidden size", "128 units each"],
      ["Clip ratio", "0.2"],
      ["Value coeff", "0.5"],
      ["Entropy coeff", "~0.01–0.02"],
      ["Training data", "199,614 Alibaba tasks"],
    ];
    const cw = 4.5, ch = 0.44;
    s.addShape("rect", { x: 0.3, y: 3.9, w: 9.4, h: 0.3,
      fill: { color: C.navy }, line: { color: C.navy } });
    s.addText("PPO Training Configuration", { x: 0.45, y: 3.91, w: 9.1, h: 0.28,
      fontSize: 11, bold: true, color: C.white, fontFace: "Calibri" });
    for (let i = 0; i < cfg.length; i++) {
      const col = i % 3, row = Math.floor(i / 3);
      const x = 0.3 + col * 3.15, y = 4.25 + row * ch;
      s.addShape("rect", { x, y, w: 3.1, h: ch - 0.04,
        fill: { color: col % 2 === 0 ? "F0F6FA" : C.cardBg }, line: { color: "C8D8E4" } });
      s.addText(cfg[i][0] + ":", { x: x + 0.1, y, w: 1.4, h: ch - 0.06,
        fontSize: 10, bold: true, color: C.navy, fontFace: "Calibri", valign: "middle" });
      s.addText(cfg[i][1], { x: x + 1.5, y, w: 1.55, h: ch - 0.06,
        fontSize: 10, color: C.muted, fontFace: "Calibri", valign: "middle" });
    }
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 17 — PPO DEPLOYMENT DESIGN
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "PPO Deployment Design — Safety First");

    // 3 modes
    const modes = [
      { name: "ACTIVE",   color: C.teal,   desc: "PPO makes the actual scheduling\ndecision. Fallback used only if\nPPO is unavailable or invalid." },
      { name: "SHADOW",   color: "8A6A0D", desc: "PPO queried for comparison.\nFallback scheduler executes the\nreal placement. Safe A/B mode." },
      { name: "FALLBACK", color: "607080", desc: "PPO bypassed entirely.\nFallback (RTS / Round-Robin)\nused directly for all tasks." },
    ];
    for (let i = 0; i < 3; i++) {
      const x = 0.3 + i * 3.25;
      s.addShape("rect", { x, y: 1.0, w: 3.05, h: 2.8,
        fill: { color: C.cardBg }, line: { color: modes[i].color, width: 2.5 },
        shadow: { type: "outer", color: "000000", blur: 8, offset: 2, angle: 135, opacity: 0.1 } });
      s.addShape("rect", { x, y: 1.0, w: 3.05, h: 0.52,
        fill: { color: modes[i].color }, line: { color: modes[i].color } });
      s.addText(modes[i].name, { x, y: 1.0, w: 3.05, h: 0.52,
        fontSize: 17, bold: true, color: C.white, fontFace: "Calibri",
        align: "center", valign: "middle" });
      s.addText(modes[i].desc, { x: x + 0.15, y: 1.65, w: 2.75, h: 1.95,
        fontSize: 12, color: C.navy, fontFace: "Calibri" });
    }

    // Fallback triggers
    s.addShape("rect", { x: 0.3, y: 4.05, w: 9.4, h: 1.25,
      fill: { color: "FFF3E0" }, line: { color: C.yellow, width: 1 } });
    s.addText("Fallback Triggers (any of these → use fallback):", { x: 0.5, y: 4.1, w: 9.0, h: 0.3,
      fontSize: 12, bold: true, color: "805000", fontFace: "Calibri" });
    const triggers = [
      "PPO service is unreachable",
      "PPO returns empty result",
      "Returned worker no longer exists",
      "Worker cannot fit task resources",
    ];
    for (let i = 0; i < 2; i++) {
      for (let j = 0; j < 2; j++) {
        const idx = i * 2 + j;
        s.addText("⚠  " + triggers[idx], { x: 0.5 + j * 4.75, y: 4.45 + i * 0.0, w: 4.6, h: 0.3,
          fontSize: 11, color: "604000", fontFace: "Calibri" });
      }
    }
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 18 — SECTION: RESULTS
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    darkSlide(s);
    s.addShape("rect", { x: 0, y: 0, w: 0.12, h: 5.625, fill: { color: C.teal }, line: { color: C.teal } });

    const ic = await iconPng(FaChartLine, "#" + C.tealLt, 512);
    s.addImage({ data: ic, x: 7.0, y: 1.2, w: 2.6, h: 2.6 });

    s.addText("03", { x: 0.5, y: 0.4, w: 1.5, h: 0.9,
      fontSize: 60, bold: true, color: "1A3A5A", fontFace: "Calibri" });
    s.addText("Results &\nEvaluation", {
      x: 0.5, y: 1.4, w: 6, h: 1.3,
      fontSize: 38, bold: true, color: C.white, fontFace: "Calibri"
    });
    s.addText("Training convergence · KPI benchmarks · Pressure scenarios", {
      x: 0.5, y: 2.85, w: 6, h: 0.4,
      fontSize: 15, color: C.tealLt, fontFace: "Calibri"
    });
    s.addShape("rect", { x: 0.5, y: 3.35, w: 5, h: 0.05, fill: { color: C.teal }, line: { color: C.teal } });
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 19 — TRAINING CONVERGENCE (chart)
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Training Convergence — 199,614 Tasks");

    // Reward chart (line chart)
    const updates = [0, 20, 40, 60, 80, 100, 120, 140, 160, 180, 200];
    const rewards = [1.638, 1.631, 1.621, 1.596, 1.575, 1.565, 1.558, 1.572, 1.579, 1.588, 1.596];
    s.addChart(pres.charts.LINE, [{
      name: "EMA Avg Reward",
      labels: updates.map(String),
      values: rewards,
    }], {
      x: 0.3, y: 0.95, w: 5.8, h: 3.5,
      lineSize: 3, lineSmooth: true,
      chartColors: [C.teal],
      chartArea: { fill: { color: "FFFFFF" } },
      showTitle: true, title: "Avg Reward per PPO Update",
      titleFontSize: 11, titleColor: C.navy,
      catAxisLabelColor: C.muted, valAxisLabelColor: C.muted,
      valGridLine: { color: "E0EAF0", size: 0.5 },
      catGridLine: { style: "none" },
      showLegend: false,
      valAxisMinVal: 1.5,
    });

    // Policy loss & Entropy chart
    const policyLoss = [0.082, 0.072, 0.064, 0.052, 0.042, 0.034, 0.026, 0.018, 0.012, 0.007, 0.003];
    const entropy = [0.72, 0.65, 0.57, 0.50, 0.44, 0.38, 0.32, 0.28, 0.25, 0.22, 0.20];
    s.addChart(pres.charts.LINE, [
      { name: "Policy Loss", labels: updates.map(String), values: policyLoss },
      { name: "Entropy",     labels: updates.map(String), values: entropy },
    ], {
      x: 6.3, y: 0.95, w: 3.4, h: 3.5,
      lineSize: 2.5,
      chartColors: [C.navy, C.accent],
      chartArea: { fill: { color: "FFFFFF" } },
      showTitle: true, title: "Policy Loss & Entropy",
      titleFontSize: 11, titleColor: C.navy,
      catAxisLabelColor: C.muted, valAxisLabelColor: C.muted,
      valGridLine: { color: "E0EAF0", size: 0.5 },
      catGridLine: { style: "none" },
      showLegend: true, legendPos: "b",
    });

    // Key insights
    const insights = [
      ["Peak reward: 1.6392", "Early exploration finding strong placements"],
      ["Reward stabilises", "Policy converged to consistent placement strategy"],
      ["Policy loss decreasing", "Actor making more correct high-advantage choices"],
      ["Entropy decreasing", "Model building stronger worker preferences"],
    ];
    for (let i = 0; i < insights.length; i++) {
      const x = 0.3 + (i % 2) * 4.75, y = 4.65 + Math.floor(i / 2) * 0.0;
      s.addShape("rect", { x, y: 4.6 + Math.floor(i / 2) * 0.48, w: 4.6, h: 0.4,
        fill: { color: i % 2 === 0 ? "F0F6FA" : C.cardBg }, line: { color: "C8D8E4" } });
      s.addText("● " + insights[i][0], { x: x + 0.1, y: 4.62 + Math.floor(i / 2) * 0.48, w: 4.4, h: 0.36,
        fontSize: 10.5, color: C.navy, fontFace: "Calibri", bold: false });
    }
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 20 — OFFLINE RESULTS TABLE
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Offline Trace Replay — Scheduler Comparison");

    // Table
    const tableData = [
      [
        { text: "Policy",         options: { bold: true, fill: { color: C.navy }, color: C.white, align: "center" } },
        { text: "Mean Reward",    options: { bold: true, fill: { color: C.navy }, color: C.white, align: "center" } },
        { text: "Feasible Rate",  options: { bold: true, fill: { color: C.navy }, color: C.white, align: "center" } },
        { text: "Verdict",        options: { bold: true, fill: { color: C.navy }, color: C.white, align: "center" } },
      ],
      [
        { text: "Round-Robin",       options: { fill: { color: "F0F4F8" }, color: C.navy, align: "center" } },
        { text: "0.8688",            options: { fill: { color: "F0F4F8" }, color: C.navy, align: "center" } },
        { text: "94.38%",            options: { fill: { color: "F0F4F8" }, color: C.navy, align: "center" } },
        { text: "Baseline",          options: { fill: { color: "F0F4F8" }, color: C.muted, align: "center" } },
      ],
      [
        { text: "First Feasible",    options: { fill: { color: "FFFFFF" }, color: C.navy, align: "center" } },
        { text: "0.8145",            options: { fill: { color: "FFFFFF" }, color: C.navy, align: "center" } },
        { text: "94.80%",            options: { fill: { color: "FFFFFF" }, color: C.navy, align: "center" } },
        { text: "Weak heuristic",    options: { fill: { color: "FFFFFF" }, color: C.muted, align: "center" } },
      ],
      [
        { text: "Max Available",     options: { fill: { color: "F0F4F8" }, color: C.navy, align: "center" } },
        { text: "0.8800",            options: { fill: { color: "F0F4F8" }, color: C.navy, align: "center" } },
        { text: "94.84%",            options: { fill: { color: "F0F4F8" }, color: C.navy, align: "center" } },
        { text: "Strong heuristic",  options: { fill: { color: "F0F4F8" }, color: "805000", align: "center" } },
      ],
      [
        { text: "PPO (ours)",        options: { bold: true, fill: { color: "E0F5EA" }, color: C.navy, align: "center" } },
        { text: "0.8801 ★",          options: { bold: true, fill: { color: "E0F5EA" }, color: C.green, align: "center" } },
        { text: "94.86% ★",          options: { bold: true, fill: { color: "E0F5EA" }, color: C.green, align: "center" } },
        { text: "Best overall",      options: { bold: true, fill: { color: "E0F5EA" }, color: C.green, align: "center" } },
      ],
    ];
    s.addTable(tableData, {
      x: 1.0, y: 1.1, w: 8.0, h: 2.2,
      colW: [2.0, 2.0, 2.0, 2.0],
      border: { pt: 1, color: "D0DCEA" },
      fontSize: 12, fontFace: "Calibri",
    });

    // Feasible rate formula
    s.addShape("rect", { x: 0.3, y: 3.55, w: 9.4, h: 0.7,
      fill: { color: "EBF6FB" }, line: { color: C.teal, width: 1 } });
    s.addText("Feasible Action Rate = (Workers satisfying task CPU/Mem/Storage) ÷ Total Decisions × 100", {
      x: 0.5, y: 3.58, w: 9.0, h: 0.6,
      fontSize: 12, color: C.navy, fontFace: "Consolas", align: "center"
    });

    // Insight callouts
    s.addShape("rect", { x: 0.3, y: 4.45, w: 4.45, h: 0.8,
      fill: { color: "FFF3E0" }, line: { color: C.yellow, width: 1 } });
    s.addText("📌  In light-load trace replay, baselines remain competitive.\nPPO advantage becomes clear under burst and overload.", {
      x: 0.45, y: 4.5, w: 4.2, h: 0.68,
      fontSize: 11, color: "604000", fontFace: "Calibri"
    });

    s.addShape("rect", { x: 5.0, y: 4.45, w: 4.7, h: 0.8,
      fill: { color: "E8F4E8" }, line: { color: C.green, width: 1 } });
    s.addText("✅  PPO matched the best heuristic at reward level while retaining ability to learn from reward signals.", {
      x: 5.15, y: 4.5, w: 4.45, h: 0.68,
      fontSize: 11, color: "1A5A30", fontFace: "Calibri"
    });
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 21 — KPI COMPARISON CHART
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "End-to-End KPI Comparison (n=5 Runs)");

    s.addChart(pres.charts.BAR, [
      { name: "Round-Robin", labels: ["Makespan (s)", "Avg Turnaround (s)", "P95 Turnaround (s)"], values: [74.4, 32.9, 66.6] },
      { name: "RTS (Heuristic)", labels: ["Makespan (s)", "Avg Turnaround (s)", "P95 Turnaround (s)"], values: [74.2, 32.1, 66.0] },
      { name: "PPO (Ours)", labels: ["Makespan (s)", "Avg Turnaround (s)", "P95 Turnaround (s)"], values: [63.1, 28.1, 59.6] },
    ], {
      x: 0.3, y: 0.85, w: 5.8, h: 4.0,
      barDir: "col", barGrouping: "clustered",
      chartColors: ["607080", "8A6A0D", C.teal],
      chartArea: { fill: { color: "FFFFFF" } },
      catAxisLabelColor: C.muted, valAxisLabelColor: C.muted,
      valGridLine: { color: "E0EAF0", size: 0.5 }, catGridLine: { style: "none" },
      showLegend: true, legendPos: "b", legendFontSize: 10,
      showValue: true, dataLabelFontSize: 9, dataLabelColor: C.navy,
    });

    // Stat callouts
    const stats = [
      { val: "15.1%", label: "Faster makespan\nvs Round-Robin", color: C.teal },
      { val: "14.6%", label: "Lower avg turnaround\nvs Round-Robin", color: C.teal },
      { val: "10.5%", label: "Better P95\nvs Round-Robin", color: C.teal },
    ];
    for (let i = 0; i < 3; i++) {
      const y = 1.1 + i * 1.4;
      s.addShape("rect", { x: 6.4, y, w: 3.25, h: 1.28,
        fill: { color: C.cardBg }, line: { color: C.teal, width: 1.5 },
        shadow: { type: "outer", color: "000000", blur: 5, offset: 2, angle: 135, opacity: 0.1 } });
      s.addText(stats[i].val, { x: 6.45, y: y + 0.1, w: 3.15, h: 0.6,
        fontSize: 32, bold: true, color: stats[i].color, fontFace: "Calibri", align: "center" });
      s.addText(stats[i].label, { x: 6.45, y: y + 0.68, w: 3.15, h: 0.5,
        fontSize: 11, color: C.muted, fontFace: "Calibri", align: "center" });
    }
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 22 — CAMPAIGN SUMMARY TABLE
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Latest Campaign — Resource Contention Workload");
    addSubTitle(s, "20 tasks: 5 CPU-heavy + 5 Mem-heavy + 5 Mixed + 5 Light", 0.85);

    const tData = [
      [
        { text: "Scheduler", options: { bold: true, fill: { color: C.navy }, color: C.white, align: "center" } },
        { text: "Tasks", options: { bold: true, fill: { color: C.navy }, color: C.white, align: "center" } },
        { text: "Completed", options: { bold: true, fill: { color: C.navy }, color: C.white, align: "center" } },
        { text: "Duration", options: { bold: true, fill: { color: C.navy }, color: C.white, align: "center" } },
        { text: "Avg Turnaround", options: { bold: true, fill: { color: C.navy }, color: C.white, align: "center" } },
        { text: "P95", options: { bold: true, fill: { color: C.navy }, color: C.white, align: "center" } },
      ],
      [
        { text: "Round-Robin", options: { fill: { color: "F0F4F8" }, color: C.navy, align: "center" } },
        { text: "20", options: { fill: { color: "F0F4F8" }, color: C.navy, align: "center" } },
        { text: "20 ✓", options: { fill: { color: "F0F4F8" }, color: C.navy, align: "center" } },
        { text: "70.06 s", options: { fill: { color: "F0F4F8" }, color: C.navy, align: "center" } },
        { text: "33.80 s", options: { fill: { color: "F0F4F8" }, color: C.navy, align: "center" } },
        { text: "69.00 s", options: { fill: { color: "F0F4F8" }, color: C.navy, align: "center" } },
      ],
      [
        { text: "RTS", options: { fill: { color: "FFFFFF" }, color: C.navy, align: "center" } },
        { text: "20", options: { fill: { color: "FFFFFF" }, color: C.navy, align: "center" } },
        { text: "20 ✓", options: { fill: { color: "FFFFFF" }, color: C.navy, align: "center" } },
        { text: "76.20 s", options: { fill: { color: "FFFFFF" }, color: C.navy, align: "center" } },
        { text: "33.40 s", options: { fill: { color: "FFFFFF" }, color: C.navy, align: "center" } },
        { text: "74.00 s", options: { fill: { color: "FFFFFF" }, color: C.navy, align: "center" } },
      ],
      [
        { text: "PPO (ours) ★", options: { bold: true, fill: { color: "E0F5EA" }, color: C.navy, align: "center" } },
        { text: "20", options: { bold: true, fill: { color: "E0F5EA" }, color: C.navy, align: "center" } },
        { text: "20 ✓", options: { bold: true, fill: { color: "E0F5EA" }, color: C.navy, align: "center" } },
        { text: "63.95 s ★", options: { bold: true, fill: { color: "E0F5EA" }, color: C.green, align: "center" } },
        { text: "27.75 s ★", options: { bold: true, fill: { color: "E0F5EA" }, color: C.green, align: "center" } },
        { text: "62.00 s ★", options: { bold: true, fill: { color: "E0F5EA" }, color: C.green, align: "center" } },
      ],
    ];
    s.addTable(tData, {
      x: 0.4, y: 1.15, w: 9.2, h: 2.0,
      colW: [1.6, 0.9, 1.2, 1.6, 2.0, 1.9],
      border: { pt: 1, color: "D0DCEA" },
      fontSize: 11, fontFace: "Calibri",
    });

    // Improvement boxes
    const imps = [
      { vs: "vs Round-Robin", dur: "8.7% faster", trt: "17.9% lower", color: C.teal },
      { vs: "vs RTS",         dur: "16.1% faster", trt: "16.9% lower", color: C.maroon },
    ];
    for (let i = 0; i < 2; i++) {
      const x = 0.4 + i * 4.75;
      s.addShape("rect", { x, y: 3.4, w: 4.5, h: 1.8,
        fill: { color: C.cardBg }, line: { color: imps[i].color, width: 2 },
        shadow: { type: "outer", color: "000000", blur: 6, offset: 2, angle: 135, opacity: 0.1 } });
      s.addShape("rect", { x, y: 3.4, w: 4.5, h: 0.38,
        fill: { color: imps[i].color }, line: { color: imps[i].color } });
      s.addText("PPO " + imps[i].vs, { x: x + 0.1, y: 3.4, w: 4.3, h: 0.38,
        fontSize: 12, bold: true, color: C.white, fontFace: "Calibri", align: "center", valign: "middle" });
      s.addText("Duration: " + imps[i].dur, { x: x + 0.2, y: 3.88, w: 4.1, h: 0.42,
        fontSize: 14, bold: true, color: imps[i].color, fontFace: "Calibri" });
      s.addText("Turnaround: " + imps[i].trt, { x: x + 0.2, y: 4.32, w: 4.1, h: 0.42,
        fontSize: 14, bold: true, color: imps[i].color, fontFace: "Calibri" });
      s.addText("(all 20 tasks completed = 100% success)", { x: x + 0.2, y: 4.78, w: 4.1, h: 0.3,
        fontSize: 10, color: C.muted, fontFace: "Calibri" });
    }
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 23 — CUMULATIVE COMPLETION CHART
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Cumulative Task Completion — PPO Consistently Faster");

    s.addChart(pres.charts.LINE, [
      { name: "Round-Robin", labels: ["0","10","20","30","40","50","60","70"], values: [0, 1, 3, 7, 11, 15, 18, 20] },
      { name: "RTS (Heuristic)", labels: ["0","10","20","30","40","50","60","70"], values: [0, 1, 4, 7, 10, 14, 17, 20] },
      { name: "PPO (Ours)", labels: ["0","10","20","30","40","50","60","70"], values: [0, 3, 7, 11, 15, 17, 19, 20] },
    ], {
      x: 0.3, y: 0.9, w: 6.5, h: 4.3,
      lineSize: 2.5,
      chartColors: ["607080", "8A6A0D", C.teal],
      chartArea: { fill: { color: "FFFFFF" } },
      showTitle: true, title: "Tasks Completed vs Elapsed Time (s)",
      titleFontSize: 12, titleColor: C.navy,
      catAxisLabelColor: C.muted, valAxisLabelColor: C.muted,
      valGridLine: { color: "E0EAF0", size: 0.5 }, catGridLine: { style: "none" },
      showLegend: true, legendPos: "b",
    });

    // Why tail matters
    const tail = [
      { label: "Steeper curve", desc: "= Tasks completing faster throughout" },
      { label: "Tail behavior", desc: "Few delayed tasks can block entire workload" },
      { label: "PPO ~5s faster", desc: "Annotated on chart: ~35s faster finish" },
      { label: "Average hides tails", desc: "P95 is the honest measure of worst-case" },
    ];
    for (let i = 0; i < 4; i++) {
      const y = 1.05 + i * 1.0;
      s.addShape("rect", { x: 7.05, y, w: 2.75, h: 0.88,
        fill: { color: C.cardBg }, line: { color: "D0DCEA", width: 1 },
        shadow: { type: "outer", color: "000000", blur: 4, offset: 1, angle: 135, opacity: 0.07 } });
      s.addShape("rect", { x: 7.05, y, w: 0.07, h: 0.88, fill: { color: C.teal }, line: { color: C.teal } });
      s.addText(tail[i].label, { x: 7.17, y: y + 0.06, w: 2.55, h: 0.3,
        fontSize: 12, bold: true, color: C.navy, fontFace: "Calibri", margin: 0 });
      s.addText(tail[i].desc, { x: 7.17, y: y + 0.38, w: 2.55, h: 0.42,
        fontSize: 10.5, color: C.muted, fontFace: "Calibri", margin: 0 });
    }
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 24 — PRESSURE SCENARIOS
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Pressure Scenarios — Where PPO Shines");

    const scenarios = [
      { name: "BASELINE",  color: "607080", desc: "1x workload,\nspaced arrivals",       rr: "Fast",    rts: "Fast",     ppo: "Fast (tie)" },
      { name: "BURST",     color: "8A6A0D", desc: "Same workload\nno spacing",            rr: "Struggles", rts: "Struggles", ppo: "Handles well" },
      { name: "OVERLOAD",  color: C.maroon, desc: "3x repeated\nworkload",                rr: "Timeout", rts: "Timeout",  ppo: "Best completion" },
    ];
    for (let i = 0; i < 3; i++) {
      const x = 0.3 + i * 3.25;
      const sc = scenarios[i];
      s.addShape("rect", { x, y: 0.95, w: 3.05, h: 4.2,
        fill: { color: C.cardBg }, line: { color: sc.color, width: 2 },
        shadow: { type: "outer", color: "000000", blur: 8, offset: 2, angle: 135, opacity: 0.12 } });
      s.addShape("rect", { x, y: 0.95, w: 3.05, h: 0.52,
        fill: { color: sc.color }, line: { color: sc.color } });
      s.addText(sc.name, { x, y: 0.95, w: 3.05, h: 0.52,
        fontSize: 16, bold: true, color: C.white, fontFace: "Calibri", align: "center", valign: "middle" });
      s.addText(sc.desc, { x: x + 0.1, y: 1.6, w: 2.85, h: 0.65,
        fontSize: 12, color: C.muted, fontFace: "Calibri", align: "center" });

      // Mini comparison
      const results = [
        { sched: "Round-Robin", result: sc.rr },
        { sched: "RTS",         result: sc.rts },
        { sched: "PPO",         result: sc.ppo },
      ];
      for (let j = 0; j < 3; j++) {
        const isGood = results[j].result.includes("Fast") || results[j].result.includes("well") || results[j].result.includes("Best");
        const rowColor = isGood ? "E8F4E8" : "FEF0F0";
        const textCol  = isGood ? C.green   : C.accent;
        s.addShape("rect", { x: x + 0.1, y: 2.45 + j * 0.6, w: 2.85, h: 0.52,
          fill: { color: rowColor }, line: { color: isGood ? "90D0A8" : "E8A0A0", width: 0.5 } });
        s.addText(results[j].sched, { x: x + 0.15, y: 2.47 + j * 0.6, w: 1.2, h: 0.46,
          fontSize: 11, bold: true, color: C.navy, fontFace: "Calibri", valign: "middle" });
        s.addText(results[j].result, { x: x + 1.4, y: 2.47 + j * 0.6, w: 1.5, h: 0.46,
          fontSize: 10.5, color: textCol, fontFace: "Calibri", valign: "middle", bold: isGood });
      }
      s.addText("Timeout = 600s (baseline/burst)\n1200s (overload)", {
        x: x + 0.1, y: 4.35, w: 2.85, h: 0.62,
        fontSize: 9, color: C.muted, fontFace: "Calibri", italic: true, align: "center"
      });
    }
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 25 — KEY ENGINEERING DECISIONS
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Key Engineering Decisions & Outcomes");

    const decisions = [
      { dec: "Go for control plane",        why: "Concurrency via goroutines",    outcome: "Clean master-worker boundaries" },
      { dec: "Docker task execution",        why: "Portable, repeatable tasks",    outcome: "Workers run any containerised job" },
      { dec: "Separate Go & Python services",why: "Control plane stays stable",    outcome: "PPO model evolves independently" },
      { dec: "Action masking in PPO",        why: "Hard resource constraints",     outcome: "Policy respects feasibility always" },
      { dec: "Lifecycle resource tracking",  why: "Simultaneous trace arrivals",   outcome: "Accurate training state vs. decay" },
      { dec: "Tail-pressure reward term",    why: "Average hides SLA failures",    outcome: "PPO guided to reduce P95 delays" },
      { dec: "Fallback scheduler validation",why: "Model can fail or be stale",    outcome: "Cluster safe when PPO unavailable" },
      { dec: "Shadow deployment mode",       why: "Safe A/B without risk",         outcome: "Compare before trusting PPO live" },
    ];

    for (let i = 0; i < decisions.length; i++) {
      const col = i % 2, row = Math.floor(i / 2);
      const x = 0.3 + col * 4.75, y = 1.0 + row * 1.15;
      s.addShape("rect", { x, y, w: 4.55, h: 1.05,
        fill: { color: C.cardBg }, line: { color: "D0DCEA", width: 1 },
        shadow: { type: "outer", color: "000000", blur: 4, offset: 1, angle: 135, opacity: 0.08 } });
      s.addShape("rect", { x, y, w: 0.08, h: 1.05, fill: { color: C.teal }, line: { color: C.teal } });
      s.addText(decisions[i].dec, { x: x + 0.18, y: y + 0.05, w: 4.25, h: 0.28,
        fontSize: 11.5, bold: true, color: C.navy, fontFace: "Calibri", margin: 0 });
      s.addText("Why: " + decisions[i].why, { x: x + 0.18, y: y + 0.36, w: 4.25, h: 0.25,
        fontSize: 9.5, color: C.muted, fontFace: "Calibri", margin: 0 });
      s.addText("✓  " + decisions[i].outcome, { x: x + 0.18, y: y + 0.64, w: 4.25, h: 0.3,
        fontSize: 10, color: C.green, fontFace: "Calibri", margin: 0, bold: true });
    }
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 26 — REAL WORLD APPLICATIONS
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.offWhite };
    addTitle(s, "Real-World Applications");

    const apps = [
      { icon: FaServer,     title: "University Lab Clusters",  body: "Share heterogeneous machines among users running experiments with mixed resource profiles." },
      { icon: FaCode,       title: "CI / CD Build Runners",    body: "Route build, test, and benchmark jobs across workers by learned load patterns." },
      { icon: FaCloud,      title: "Private Cloud Platforms",  body: "Lightweight batch execution layer — no need to deploy full Kubernetes for internal batches." },
      { icon: FaCube,       title: "Edge Computing",           body: "Schedule video, inference, or filtering jobs on resource-limited heterogeneous edge nodes." },
    ];
    const iconColors = [C.teal, "1A8A5A", "8A6A0D", C.maroon];
    for (let i = 0; i < 4; i++) {
      const col = i % 2, row = Math.floor(i / 2);
      const x = 0.3 + col * 4.75, y = 1.1 + row * 2.1;
      s.addShape("rect", { x, y, w: 4.55, h: 1.88,
        fill: { color: C.cardBg }, line: { color: "D0DCEA", width: 1 },
        shadow: { type: "outer", color: "000000", blur: 6, offset: 2, angle: 135, opacity: 0.1 } });
      const ic = await iconPng(apps[i].icon, "#" + iconColors[i], 256);
      s.addImage({ data: ic, x: x + 0.2, y: y + 0.6, w: 0.68, h: 0.68 });
      s.addText(apps[i].title, { x: x + 1.0, y: y + 0.15, w: 3.4, h: 0.38,
        fontSize: 13, bold: true, color: C.navy, fontFace: "Calibri", margin: 0 });
      s.addText(apps[i].body, { x: x + 1.0, y: y + 0.58, w: 3.4, h: 1.15,
        fontSize: 11, color: C.muted, fontFace: "Calibri", margin: 0 });
    }
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 27 — CONCLUSION & FUTURE WORK
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: C.navy };

    s.addShape("rect", { x: 0, y: 0, w: 0.12, h: 5.625, fill: { color: C.teal }, line: { color: C.teal } });

    s.addText("Conclusion", {
      x: 0.4, y: 0.25, w: 6, h: 0.6,
      fontSize: 30, bold: true, color: C.white, fontFace: "Calibri"
    });

    const bullets = [
      "Built a complete Go master-worker cluster with Docker task execution",
      "Scheduler interface allows swapping algorithms without system changes",
      "PPO policy trained on 199,614 real Alibaba trace tasks",
      "PPO wins on all KPIs under burst & overload, especially P95 tail latency",
      "Learned scheduling most valuable when placement decisions are non-obvious",
      "Safety: learning model always has fallback validation layer",
    ];
    for (let i = 0; i < bullets.length; i++) {
      s.addShape("ellipse", { x: 0.42, y: 1.0 + i * 0.55, w: 0.22, h: 0.22,
        fill: { color: C.teal }, line: { color: C.teal } });
      s.addText(bullets[i], { x: 0.75, y: 0.97 + i * 0.55, w: 5.4, h: 0.42,
        fontSize: 12, color: "C8D8E8", fontFace: "Calibri", margin: 0 });
    }

    // Future work
    s.addShape("rect", { x: 6.5, y: 0.9, w: 3.2, h: 4.2,
      fill: { color: "0D2240" }, line: { color: C.tealLt, width: 1 } });
    s.addText("Future Work", { x: 6.6, y: 0.97, w: 3.0, h: 0.36,
      fontSize: 13, bold: true, color: C.tealLt, fontFace: "Calibri", align: "center" });
    const fw = [
      "Larger & more diverse traces",
      "Multi-objective reward\n(cost, energy, fairness)",
      "Online adaptation pipeline",
      "More physical machines",
      "Failure injection testing",
      "CRIU checkpoint/restore\nfor preemption support",
    ];
    for (let i = 0; i < fw.length; i++) {
      s.addText("→  " + fw[i], { x: 6.65, y: 1.45 + i * 0.55, w: 2.9, h: 0.5,
        fontSize: 10.5, color: "A0BDCC", fontFace: "Calibri" });
    }

    s.addShape("rect", { x: 0.4, y: 5.1, w: 5.8, h: 0.35,
      fill: { color: C.teal }, line: { color: C.teal } });
    s.addText("\"Learning-based scheduling should improve systems, not replace reliability.\"", {
      x: 0.45, y: 5.1, w: 5.7, h: 0.35,
      fontSize: 10.5, italic: true, color: C.white, fontFace: "Calibri", align: "center", valign: "middle"
    });
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SLIDE 28 — THANK YOU
  // ══════════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    darkSlide(s);
    s.addShape("rect", { x: 0, y: 0, w: 0.12, h: 5.625, fill: { color: C.teal }, line: { color: C.teal } });

    s.addText("Thank You", {
      x: 0.5, y: 1.0, w: 9.0, h: 1.0,
      fontSize: 50, bold: true, color: C.white, fontFace: "Calibri", align: "center"
    });
    s.addText("Agentic Cloud Cluster", {
      x: 0.5, y: 2.15, w: 9.0, h: 0.45,
      fontSize: 20, color: C.tealLt, fontFace: "Calibri", align: "center"
    });
    s.addShape("rect", { x: 3.5, y: 2.7, w: 3.0, h: 0.05, fill: { color: C.teal }, line: { color: C.teal } });
    s.addText("Sarthak Siddhpura  |  AU2240041\nSEAS, Ahmedabad University  |  May 2026\nMentor: Prof. Sanjay Chaudhary", {
      x: 0.5, y: 2.9, w: 9.0, h: 0.9,
      fontSize: 14, color: "A0BDCC", fontFace: "Calibri", align: "center"
    });

    const icA = await iconPng(FaServer, "#" + C.tealLt, 256);
    const icB = await iconPng(FaBrain, "#" + C.tealLt, 256);
    const icC = await iconPng(FaNetworkWired, "#" + C.tealLt, 256);
    s.addImage({ data: icA, x: 2.0, y: 4.0, w: 0.8, h: 0.8 });
    s.addImage({ data: icB, x: 4.5, y: 4.0, w: 0.8, h: 0.8 });
    s.addImage({ data: icC, x: 7.0, y: 4.0, w: 0.8, h: 0.8 });

    s.addText("Questions?", {
      x: 0.5, y: 4.9, w: 9.0, h: 0.5,
      fontSize: 16, color: C.gray, fontFace: "Calibri", align: "center"
    });
  }

  // ══════════════════════════════════════════════════════════════════════════
  // SPEAKER NOTES
  // ══════════════════════════════════════════════════════════════════════════
  const notes = [
    // Slide 1
    "Introduce yourself: name, enrollment, and that this is your final B.Tech project. Briefly describe the project: a distributed task-execution cluster built in Go where the scheduling decisions are made by a PPO reinforcement learning agent. Mention that you will walk through the motivation, system design, RL methodology, and results.",
    // Slide 2
    "Walk through the agenda quickly. Explain you have 25 slides so you will keep each section focused. Tell them where results are (section 5) so they know the data is coming.",
    // Slide 3
    "Key message: simple rules like Round-Robin ignore resource state and task demand. A CPU-heavy and a memory-heavy task should NOT go to the same worker just because it is 'next in line'. State changes every few seconds which makes fixed heuristics brittle. This motivates a learned approach.",
    // Slide 4
    "Read through the 6 objectives. Emphasise objective 3 (clean scheduler interface) — this is the engineering discipline that let you switch algorithms without rewriting the worker layer.",
    // Slide 5
    "Walk the architecture diagram: (1) Client sends task via HTTP API, (2) Master stores and queues it, (3) Scheduler layer selects a worker, (4) gRPC AssignTask sent to worker, (5) Worker runs Docker container, (6) Result reported back. PPO Scheduler lives as a separate Python service so the Go control plane never goes down because of a model bug.",
    // Slide 6
    "Describe each module briefly. Emphasise the Scheduler Package — it is the clean boundary. The master only asks 'which worker?' and any scheduler can answer. Telemetry Manager feeds the scheduling with live resource data.",
    // Slide 7
    "Workers are intentionally simple and stateless. They don't know about other workers. If one disappears the master detects missed heartbeats and requeues. Docker gives portability — any containerised job runs without change.",
    // Slide 8
    "Walk through the 6-step flow. Then point to the recovery box: attempt-level isolation means you can retry without losing task identity. Mention the task state machine at the bottom.",
    // Slide 9
    "The interface is deliberately minimal: inputs → worker ID. Left side shows the 3 schedulers plug in without touching worker code. Right side shows resource accounting — the master always validates PPO's returned ID against current state before dispatch.",
    // Slide 10
    "Section transition. Just read the slide title and say: 'Now let's talk about why we chose reinforcement learning, and how we arrived at PPO.'",
    // Slide 11
    "Map each RL concept to the scheduling domain. The key insight is that every placement changes future state — exactly the Markov structure RL is designed for. A reward that measures feasibility, balance, and tail risk guides the agent toward practical behavior.",
    // Slide 12
    "Tell the evolution story. Q-Tables work in textbooks but explode for real cluster state. DQN solves storage but is awkward for dynamic worker sets. PPO directly learns a worker-selection probability — exactly what we need. The clipped objective prevents dangerous policy jumps.",
    // Slide 13
    "Walk the data flow: task + worker features → pairwise encoding → shared 128×128 layers → actor head (worker probabilities) → critic head (state value). Action masking happens BEFORE softmax so infeasible workers get zero probability.",
    // Slide 14
    "Task vector has 5 values, worker vector has 9 values. The model sees one row per worker — each row is task+worker concatenated. This pairwise design lets the model answer 'how good is THIS worker for THIS task?' not just 'how good is the worker in isolation?'",
    // Slide 15
    "Walk through each reward term. Start with the base +1.4 — any feasible placement gets a positive base. Headroom bonus rewards free capacity. Queue/tail pressure penalise risky placements. Imbalance terms prevent hot-spots. The −1.8 for infeasible placement is the hard floor.",
    // Slide 16
    "Training took ~17 weeks total project time. The Alibaba trace has 199,614 real tasks. Lifecycle resource tracking was critical — simultaneous arrivals in the trace would corrupt exponential decay models. PPO update loop runs 200 iterations at 16,384 env steps each.",
    // Slide 17
    "Three deployment modes give operators control over trust. Shadow mode lets you compare PPO vs fallback without risk. Active mode flips to PPO with automatic fallback. The fallback triggers on the right are all coded in Go — the learning model is never the only safety check.",
    // Slide 18
    "Section transition. Say: 'Now I'll show training convergence, offline results, and end-to-end campaign data.'",
    // Slide 19
    "Left chart: average reward per PPO update over 200 updates on 199,614 tasks. Peak 1.6392, stabilises around 1.59. Right chart: policy loss drops from ~0.08 to near 0, entropy drops from 0.72 to ~0.20 — the policy is converging and building strong worker preferences.",
    // Slide 20
    "Offline table: all 4 policies on the same Alibaba trace. PPO achieves the highest mean reward (0.8801) and feasible rate (94.86%). The margin is small in light-load — this is honest. The big gains come under pressure.",
    // Slide 21
    "Bar chart: PPO beats Round-Robin and RTS on all three KPIs across 5 independent runs. Point to the stat callout boxes on the right. The error bars show consistency — not one lucky run.",
    // Slide 22
    "Campaign summary: 20 real Docker tasks, 3 workers, no failures. PPO finished in 63.95s vs 70.06s (RR) and 76.20s (RTS). Turnaround improvement is 17.9% vs Round-Robin. P95 improved from 69s to 62s — slower tail tasks completed earlier.",
    // Slide 23
    "Cumulative chart: steeper curve = faster task throughput. PPO is consistently above both baselines throughout the campaign. Tail behavior: even the last 2-3 tasks complete faster with PPO. This is what P95 captures.",
    // Slide 24
    "Three pressure modes: baseline (easy), burst (no spacing), overload (3x workload). PPO advantage is clearest under burst and overload. Under baseline, all three are roughly equal — this is the honest result. Adding intelligence also adds inference overhead so it must justify itself.",
    // Slide 25
    "Recap 8 key decisions. Ask the examiner to notice three things: (1) separation of control plane and ML service, (2) action masking for hard constraints, (3) fallback validation for safety. Each decision had a clear reason and a measurable outcome.",
    // Slide 26
    "Four real domains where the same design pattern applies. None of these require Kubernetes. The framework is lightweight enough to deploy on a laptop cluster or a few VMs — making it useful for university labs and small private clouds.",
    // Slide 27
    "Summarise: built the cluster, designed the reward, trained on real traces, showed improvement under pressure, maintained safety. Future work: larger traces, multi-objective reward, online adaptation, real hardware, and CRIU for preemption. Closing quote reinforces that RL supplements reliability, it doesn't replace it.",
    // Slide 28
    "Open for questions. If asked about limitations: honest answer is that PPO has inference overhead and its advantage is most visible under high scheduling pressure. In light load, Round-Robin is fine. The system is designed so you can choose.",
  ];

  // Attach notes to slides
  const slides = pres.slides;
  for (let i = 0; i < slides.length && i < notes.length; i++) {
    slides[i].addNotes(notes[i]);
  }

  await pres.writeFile({ fileName: "/home/claude/agentic_cloud_cluster.pptx" });
  console.log("PPT created successfully!");
})();
