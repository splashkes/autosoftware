(() => {
  const SELECTOR = "[data-sprout-logo]";
  const SVG_NS = "http://www.w3.org/2000/svg";
  const DURATION = 30000;
  const VIEWBOX = { width: 220, height: 150 };
  const SEED = { x: 109, y: 111 };
  const LIGHT = { x: 112, y: 5 };
  const SKY = { y: 7, left: 48, right: 172 };

  class SproutLogo {
    constructor(root) {
      this.root = root;
      this.duration = Number(root.dataset.duration || DURATION);
      this.reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      this.frame = 0;
      this.startTime = 0;
      this.branches = [];
      this.buds = [];
      this.svg = null;
      this.pathGroup = null;
      this.budLayer = null;
      this.childrenByParent = new Map();
      this.seedOffset = 0;
      this.handleRestart = this.restart.bind(this);
      this.render = this.render.bind(this);
    }

    mount() {
      if (this.root.__sproutLogo) return this.root.__sproutLogo;

      this.root.__sproutLogo = this;
      this.root.addEventListener("click", this.handleRestart);
      this.resetScene();

      if (this.reduceMotion) {
        this.renderStatic();
        return this;
      }

      this.frame = requestAnimationFrame(this.render);
      return this;
    }

    destroy() {
      cancelAnimationFrame(this.frame);
      this.root.removeEventListener("click", this.handleRestart);
      this.root.innerHTML = "";
      delete this.root.__sproutLogo;
    }

    restart() {
      cancelAnimationFrame(this.frame);
      this.startTime = 0;
      this.resetScene();

      if (this.reduceMotion) {
        this.renderStatic();
        return;
      }

      this.frame = requestAnimationFrame(this.render);
    }

    resetScene() {
      this.root.innerHTML = "";
      this.root.classList.remove("is-static");
      this.startTime = 0;
      this.seedOffset = Math.floor(Math.random() * 100000);
      this.svg = createSVG("svg", {
        viewBox: `0 0 ${VIEWBOX.width} ${VIEWBOX.height}`,
        role: "presentation",
        "aria-hidden": "true",
      });

      this.svg.appendChild(this.buildDefs());
      this.svg.appendChild(this.buildScene());
      this.root.appendChild(this.svg);

      this.branches = this.buildBranches();
      this.childrenByParent = groupChildrenByParent(this.branches);
      this.decorateRootSupports();
      this.drawAmbientDrift();
    }

    buildDefs() {
      const defs = createSVG("defs");

      const blur = createSVG("filter", {
        id: "sprout-blur",
        x: "-60%",
        y: "-60%",
        width: "220%",
        height: "220%",
      });
      blur.appendChild(createSVG("feGaussianBlur", { stdDeviation: "2.8" }));
      defs.appendChild(blur);

      const softGlow = createSVG("filter", {
        id: "sprout-soft-glow",
        x: "-40%",
        y: "-40%",
        width: "180%",
        height: "180%",
      });
      softGlow.appendChild(createSVG("feGaussianBlur", { stdDeviation: "1.1", result: "blur" }));
      softGlow.appendChild(createSVG("feMerge")).append(
        createSVG("feMergeNode", { in: "blur" }),
        createSVG("feMergeNode", { in: "SourceGraphic" }),
      );
      defs.appendChild(softGlow);

      const lightGradient = createSVG("radialGradient", {
        id: "sprout-light-gradient",
        cx: "50%",
        cy: "50%",
        r: "50%",
      });
      lightGradient.append(
        createSVG("stop", { offset: "0%", "stop-color": "#f4d35e", "stop-opacity": "0.5" }),
        createSVG("stop", { offset: "58%", "stop-color": "#f4d35e", "stop-opacity": "0.14" }),
        createSVG("stop", { offset: "100%", "stop-color": "#f4d35e", "stop-opacity": "0" }),
      );
      defs.appendChild(lightGradient);

      const supportGradient = createSVG("linearGradient", {
        id: "sprout-support-gradient",
        x1: String(SEED.x),
        y1: String(SKY.y + 10),
        x2: String(SEED.x),
        y2: String(SEED.y),
        gradientUnits: "userSpaceOnUse",
      });
      supportGradient.append(
        createSVG("stop", { offset: "0%", "stop-color": "#74da8a", "stop-opacity": "0.92" }),
        createSVG("stop", { offset: "58%", "stop-color": "#6bbf72", "stop-opacity": "0.88" }),
        createSVG("stop", { offset: "100%", "stop-color": "#8a6a43", "stop-opacity": "0.94" }),
      );
      defs.appendChild(supportGradient);

      return defs;
    }

    buildScene() {
      const scene = createSVG("g");

      scene.append(
        createSVG("circle", { class: "sprout-aura", cx: LIGHT.x, cy: LIGHT.y, r: 40 }),
        createSVG("circle", { class: "sprout-light-core", cx: LIGHT.x, cy: LIGHT.y, r: 10.5 }),
        createSVG("path", {
          class: "sprout-guide",
          d: `M ${LIGHT.x} ${LIGHT.y + 8} C 111 38 110 78 ${SEED.x} ${SEED.y - 18}`,
        }),
      );

      this.pathGroup = createSVG("g");
      this.budLayer = createSVG("g");

      scene.append(
        this.pathGroup,
        this.budLayer,
        createSVG("circle", { class: "sprout-seed", cx: SEED.x, cy: SEED.y, r: 2.16 }),
      );

      return scene;
    }

    buildBranches() {
      const trunk = makeBranch({
        id: "trunk",
        isRoot: true,
        start: SEED,
        steps: 13,
        length: 3.9,
        width: 2.3,
        drift: 0.015,
        noise: 0.1,
        lightPull: 0.82,
        skySpread: 0.12,
        momentum: 0.9,
        seed: 17 + this.seedOffset,
        startAt: 0.05,
        endAt: 0.16,
        targetX: 111,
        ownLeafMass: 0.2,
        wander: 0.18,
        curl: 0.12,
      });

      const left = makeBranch({
        id: "left",
        parentId: "trunk",
        start: pointAtIndex(trunk.points, 0.24),
        steps: 12,
        length: 3.95,
        width: 1.78,
        drift: -0.82,
        noise: 0.18,
        lightPull: 0.68,
        skySpread: 0.62,
        momentum: 0.78,
        seed: 23 + this.seedOffset,
        attachAt: 0.24,
        startAt: 0.13,
        endAt: 0.34,
        targetX: 58,
        ownLeafMass: 0.55,
      });

      const right = makeBranch({
        id: "right",
        parentId: "trunk",
        start: pointAtIndex(trunk.points, 0.28),
        steps: 12,
        length: 3.85,
        width: 1.82,
        drift: 0.8,
        noise: 0.17,
        lightPull: 0.7,
        skySpread: 0.6,
        momentum: 0.8,
        seed: 29 + this.seedOffset,
        attachAt: 0.28,
        startAt: 0.14,
        endAt: 0.35,
        targetX: 162,
        ownLeafMass: 0.55,
      });

      const midLeft = makeBranch({
        id: "mid-left",
        parentId: "trunk",
        start: pointAtIndex(trunk.points, 0.46),
        steps: 10,
        length: 3.35,
        width: 1.4,
        drift: -0.3,
        noise: 0.16,
        lightPull: 0.86,
        skySpread: 0.24,
        momentum: 0.84,
        seed: 31 + this.seedOffset,
        attachAt: 0.46,
        startAt: 0.18,
        endAt: 0.38,
        targetX: 96,
        ownLeafMass: 0.5,
      });

      const midRight = makeBranch({
        id: "mid-right",
        parentId: "trunk",
        start: pointAtIndex(trunk.points, 0.5),
        steps: 10,
        length: 3.3,
        width: 1.4,
        drift: 0.28,
        noise: 0.16,
        lightPull: 0.86,
        skySpread: 0.22,
        momentum: 0.84,
        seed: 33 + this.seedOffset,
        attachAt: 0.5,
        startAt: 0.2,
        endAt: 0.4,
        targetX: 128,
        ownLeafMass: 0.5,
      });

      const crown = makeBranch({
        id: "crown",
        parentId: "trunk",
        start: pointAtIndex(trunk.points, 0.56),
        steps: 11,
        length: 3.45,
        width: 1.58,
        drift: 0.06,
        noise: 0.14,
        lightPull: 0.9,
        skySpread: 0.08,
        momentum: 0.86,
        seed: 37 + this.seedOffset,
        attachAt: 0.56,
        startAt: 0.2,
        endAt: 0.42,
        targetX: 112,
        ownLeafMass: 0.72,
      });

      const leftFine = makeBranch({
        id: "left-fine",
        parentId: "left",
        start: left.points[left.points.length - 1],
        steps: 8,
        length: 2.7,
        width: 1.08,
        drift: -0.14,
        noise: 0.24,
        lightPull: 0.92,
        skySpread: 0.76,
        momentum: 0.72,
        seed: 41 + this.seedOffset,
        attachAt: 1,
        startAt: 0.48,
        endAt: 0.84,
        fine: true,
        targetX: 46,
        ownLeafMass: 2.3,
        wander: 0.58,
        curl: 0.28,
      });

      const leftSplit = makeBranch({
        id: "left-split",
        parentId: "left",
        start: pointAtIndex(left.points, 0.56),
        steps: 8,
        length: 2.6,
        width: 0.98,
        drift: -0.48,
        noise: 0.22,
        lightPull: 0.9,
        skySpread: 0.64,
        momentum: 0.74,
        seed: 42 + this.seedOffset,
        attachAt: 0.56,
        startAt: 0.54,
        endAt: 0.82,
        fine: true,
        targetX: 70,
        ownLeafMass: 1.9,
        wander: 0.52,
        curl: 0.24,
      });

      const rightFine = makeBranch({
        id: "right-fine",
        parentId: "right",
        start: right.points[right.points.length - 1],
        steps: 8,
        length: 2.7,
        width: 1.08,
        drift: 0.12,
        noise: 0.24,
        lightPull: 0.94,
        skySpread: 0.74,
        momentum: 0.74,
        seed: 43 + this.seedOffset,
        attachAt: 1,
        startAt: 0.5,
        endAt: 0.86,
        fine: true,
        targetX: 174,
        ownLeafMass: 2.3,
        wander: 0.58,
        curl: 0.28,
      });

      const rightSplit = makeBranch({
        id: "right-split",
        parentId: "right",
        start: pointAtIndex(right.points, 0.54),
        steps: 8,
        length: 2.55,
        width: 0.98,
        drift: 0.46,
        noise: 0.22,
        lightPull: 0.9,
        skySpread: 0.64,
        momentum: 0.74,
        seed: 44 + this.seedOffset,
        attachAt: 0.54,
        startAt: 0.56,
        endAt: 0.84,
        fine: true,
        targetX: 150,
        ownLeafMass: 1.9,
        wander: 0.52,
        curl: 0.24,
      });

      const crownLeft = makeBranch({
        id: "crown-left",
        parentId: "crown",
        start: pointAtIndex(crown.points, 0.52),
        steps: 7,
        length: 2.55,
        width: 0.92,
        drift: -0.12,
        noise: 0.22,
        lightPull: 0.98,
        skySpread: 0.18,
        momentum: 0.78,
        seed: 47 + this.seedOffset,
        attachAt: 0.52,
        startAt: 0.34,
        endAt: 0.66,
        fine: true,
        targetX: 102,
        ownLeafMass: 1.7,
        wander: 0.48,
        curl: 0.2,
      });

      const crownRight = makeBranch({
        id: "crown-right",
        parentId: "crown",
        start: pointAtIndex(crown.points, 0.58),
        steps: 7,
        length: 2.55,
        width: 0.92,
        drift: 0.12,
        noise: 0.22,
        lightPull: 0.98,
        skySpread: 0.18,
        momentum: 0.78,
        seed: 49 + this.seedOffset,
        attachAt: 0.58,
        startAt: 0.36,
        endAt: 0.68,
        fine: true,
        targetX: 122,
        ownLeafMass: 1.7,
        wander: 0.48,
        curl: 0.2,
      });

      const midLeftFine = makeBranch({
        id: "mid-left-fine",
        parentId: "mid-left",
        start: pointAtIndex(midLeft.points, 0.62),
        steps: 8,
        length: 2.4,
        width: 0.86,
        drift: -0.12,
        noise: 0.2,
        lightPull: 0.96,
        skySpread: 0.18,
        momentum: 0.78,
        seed: 51 + this.seedOffset,
        attachAt: 0.62,
        startAt: 0.28,
        endAt: 0.58,
        fine: true,
        targetX: 100,
        ownLeafMass: 1.7,
        wander: 0.46,
        curl: 0.18,
      });

      const midRightFine = makeBranch({
        id: "mid-right-fine",
        parentId: "mid-right",
        start: pointAtIndex(midRight.points, 0.62),
        steps: 8,
        length: 2.4,
        width: 0.86,
        drift: 0.12,
        noise: 0.2,
        lightPull: 0.96,
        skySpread: 0.18,
        momentum: 0.78,
        seed: 53 + this.seedOffset,
        attachAt: 0.62,
        startAt: 0.3,
        endAt: 0.6,
        fine: true,
        targetX: 124,
        ownLeafMass: 1.7,
        wander: 0.46,
        curl: 0.18,
      });

      const crownCenter = makeBranch({
        id: "crown-center",
        parentId: "crown",
        start: pointAtIndex(crown.points, 0.68),
        steps: 8,
        length: 2.9,
        width: 0.98,
        drift: 0.04,
        noise: 0.16,
        lightPull: 1,
        skySpread: 0.04,
        momentum: 0.82,
        seed: 55 + this.seedOffset,
        attachAt: 0.68,
        startAt: 0.24,
        endAt: 0.5,
        fine: true,
        targetX: 112,
        ownLeafMass: 2.2,
        wander: 0.32,
        curl: 0.14,
      });

      const specs = [
        trunk,
        left,
        right,
        midLeft,
        midRight,
        crown,
        leftFine,
        leftSplit,
        rightFine,
        rightSplit,
        crownLeft,
        crownRight,
        midLeftFine,
        midRightFine,
        crownCenter,
      ];
      return specs.map((spec) => this.createBranchNode(spec));
    }

    createBranchNode(spec) {
      const branchGroup = createSVG("g", {
        class: spec.fine ? "sprout-branch is-fine" : "sprout-branch",
      });
      this.pathGroup.appendChild(branchGroup);

      const segments = [];
      let totalLength = 0;
      const segmentCount = Math.max(1, spec.points.length - 1);
      for (let index = 0; index < segmentCount; index += 1) {
        const start = spec.points[index];
        const end = spec.points[index + 1];
        const segmentLength = distance(start, end);
        const width = branchWidthAt(spec, (index + 0.5) / segmentCount, spec.widthBase);
        const segment = createSVG("path", {
          class: spec.fine ? "sprout-path is-fine" : "sprout-path",
          d: `M ${start.x.toFixed(2)} ${start.y.toFixed(2)} L ${end.x.toFixed(2)} ${end.y.toFixed(2)}`,
          "stroke-width": width.toFixed(2),
        });
        segment.style.strokeDasharray = String(segmentLength);
        segment.style.strokeDashoffset = String(segmentLength);
        branchGroup.appendChild(segment);
        segments.push({
          node: segment,
          length: segmentLength,
          startLength: totalLength,
          endLength: totalLength + segmentLength,
          position: (index + 0.5) / segmentCount,
          start,
          end,
        });
        totalLength += segmentLength;
      }

      const bud = createSVG("ellipse", {
        class: "sprout-bud",
        cx: spec.end.x,
        cy: spec.end.y,
        rx: spec.fine ? 2.2 : 2.6,
        ry: spec.fine ? 1.05 : 1.25,
        opacity: "0",
      });
      const angle = Math.atan2(spec.end.y - spec.preEnd.y, spec.end.x - spec.preEnd.x) * 180 / Math.PI;
      bud.setAttribute("transform", `rotate(${angle} ${spec.end.x} ${spec.end.y})`);
      this.budLayer.appendChild(bud);

      return {
        ...spec,
        budRx: spec.fine ? 2.2 : 2.6,
        budRy: spec.fine ? 1.05 : 1.25,
        segments,
        branchNode: branchGroup,
        budNode: bud,
        length: totalLength,
      };
    }

    drawAmbientDrift() {
      const driftPath = createSVG("path", {
        class: "sprout-drift",
        d: `M 84 118 C 80 104 77 88 80 70 C 83 50 94 30 108 18 C 88 30 74 54 71 82 C 68 96 71 109 78 122 Z`,
        opacity: "0.24",
      });
      this.svg.querySelector("g").insertBefore(driftPath, this.pathGroup);
    }

    decorateRootSupports() {
      const root = this.branches.find((branch) => branch.isRoot);
      if (!root) return;

      const fullProgress = new Map(this.branches.map((branch) => [branch.id, 1]));
      const massCache = new Map();
      const children = this.childrenByParent.get(root.id) || [];
      const supportGroup = createSVG("g", { class: "sprout-root-supports" });
      const supportLayers = [];

      children
        .slice()
        .sort((a, b) => (b.attachAt || 0) - (a.attachAt || 0))
        .forEach((child) => {
          const attachedPoint = pointAtIndex(root.points, child.attachAt || 1);
          const supportPoints = pointsUntil(root.points, attachedPoint).reverse();
          const layerPath = createSVG("path", {
            class: "sprout-root-path",
            d: pointsToCurve(supportPoints),
          });
          const length = pathLengthFromPoints(supportPoints);
          layerPath.style.strokeDasharray = String(length);
          layerPath.style.strokeDashoffset = String(length);
          supportGroup.appendChild(layerPath);

          const supportedLeafMass = resolveLeafMass(child, fullProgress, this.childrenByParent, massCache);
          supportLayers.push({
            node: layerPath,
            progressId: child.id,
            attachAt: child.attachAt || 1,
            length,
            finalWidth: 0.65 + supportedLeafMass * 0.44,
          });
        });

      this.pathGroup.insertBefore(supportGroup, root.branchNode);
      root.supportLayers = supportLayers;
    }

    renderStatic() {
      this.root.classList.add("is-static");
      const progressById = new Map(this.branches.map((branch) => [branch.id, 1]));
      const massCache = new Map();
      for (const branch of this.branches) {
        updateSupportLayers(branch, progressById, this.childrenByParent, massCache);
        for (const segment of branch.segments) {
          segment.node.style.strokeDashoffset = "0";
          segment.node.setAttribute(
            "stroke-width",
            resolveSegmentWidth(branch, segment.position, progressById, this.childrenByParent, massCache).toFixed(2),
          );
        }
        branch.budNode.setAttribute("rx", branch.budRx);
        branch.budNode.setAttribute("ry", branch.budRy);
        branch.budNode.setAttribute("opacity", "1");
      }
    }

    render(now) {
      if (!this.startTime) this.startTime = now;
      const elapsed = now - this.startTime;
      const progress = Math.min(1, elapsed / this.duration);
      const progressById = new Map();
      const massCache = new Map();

      for (const branch of this.branches) {
        const local = clamp((progress - branch.startAt) / (branch.endAt - branch.startAt), 0, 1);
        progressById.set(branch.id, local);
      }

      for (const branch of this.branches) {
        const local = progressById.get(branch.id) || 0;
        const drawLength = branch.length * local;
        updateSupportLayers(branch, progressById, this.childrenByParent, massCache);
        for (const segment of branch.segments) {
          const visible = clamp(drawLength - segment.startLength, 0, segment.length);
          segment.node.style.strokeDashoffset = String(segment.length - visible);
          segment.node.setAttribute(
            "stroke-width",
            resolveSegmentWidth(branch, segment.position, progressById, this.childrenByParent, massCache).toFixed(2),
          );
        }

        const budOpacity = smoothstep(0.82, 1, local);
        branch.budNode.setAttribute("opacity", budOpacity.toFixed(3));
        const angle = Math.atan2(branch.end.y - branch.preEnd.y, branch.end.x - branch.preEnd.x) * 180 / Math.PI;
        branch.budNode.setAttribute("rx", (branch.budRx * (0.72 + budOpacity * 0.28)).toFixed(2));
        branch.budNode.setAttribute("ry", (branch.budRy * (0.72 + budOpacity * 0.28)).toFixed(2));
        branch.budNode.setAttribute("transform", `rotate(${angle} ${branch.end.x} ${branch.end.y})`);
      }

      if (progress < 1) {
        this.frame = requestAnimationFrame(this.render);
      } else {
        this.renderStatic();
      }
    }
  }

  function init(root = document) {
    root.querySelectorAll(SELECTOR).forEach((node) => {
      if (!node.__sproutLogo) {
        new SproutLogo(node).mount();
      }
    });
  }

  function createSVG(tag, attrs = {}) {
    const node = document.createElementNS(SVG_NS, tag);
    Object.entries(attrs).forEach(([key, value]) => node.setAttribute(key, String(value)));
    return node;
  }

  function makeBranch(config) {
    const rng = mulberry32(config.seed);
    const points = [config.start];
    let current = { ...config.start };
    const skyTarget = {
      x: clamp(config.targetX ?? (LIGHT.x + config.drift * 44 + (rng() - 0.5) * 6), SKY.left, SKY.right),
      y: config.targetY ?? (SKY.y + rng() * 6),
    };
    let direction = { x: config.drift * 0.18, y: -1 };
    let wander = (rng() - 0.5) * 0.36;
    let curl = (rng() - 0.5) * 0.24;

    for (let index = 0; index < config.steps; index += 1) {
      const toSky = normalize({
        x: skyTarget.x - current.x,
        y: skyTarget.y - current.y,
      });
      wander = clamp(
        wander * 0.82 + (rng() - 0.5) * (config.wander ?? 0.52),
        -(config.wanderClamp ?? 0.96),
        config.wanderClamp ?? 0.96,
      );
      curl = clamp(
        curl * 0.84 + (rng() - 0.5) * (config.curl ?? 0.28),
        -(config.curlClamp ?? 0.52),
        config.curlClamp ?? 0.52,
      );
      const jitter = {
        x: (rng() - 0.5) * 2 * config.noise + wander + curl * (1.15 - index / Math.max(config.steps - 1, 1)),
        y: (rng() - 0.5) * config.noise * 0.65,
      };
      const spread = ((skyTarget.x - LIGHT.x) / (SKY.right - SKY.left)) * (config.skySpread ?? 0.45);
      direction = normalize({
        x: direction.x * config.momentum + toSky.x * config.lightPull + spread + config.drift * 0.34 + jitter.x,
        y: direction.y * config.momentum + toSky.y * config.lightPull - 0.32 + jitter.y,
      });

      const stepLength = config.length * (1 - index / (config.steps * 7.5));
      current = {
        x: clamp(current.x + direction.x * stepLength, 32, 188),
        y: clamp(current.y + direction.y * stepLength, 20, 128),
      };
      points.push(current);
    }

    return {
      ...config,
      widthBase: config.width,
      ownLeafMass: config.ownLeafMass || 0,
      points,
      end: points[points.length - 1],
      preEnd: points[points.length - 2],
    };
  }

  function pointAtIndex(points, ratio) {
    const index = Math.max(0, Math.min(points.length - 1, Math.round((points.length - 1) * ratio)));
    return points[index];
  }

  function pointsToCurve(points, endPoint) {
    const list = endPoint ? pointsUntil(points, endPoint) : points.slice();
    if (list.length < 2) return "";
    if (list.length === 2) {
      return `M ${list[0].x.toFixed(2)} ${list[0].y.toFixed(2)} L ${list[1].x.toFixed(2)} ${list[1].y.toFixed(2)}`;
    }

    let path = `M ${list[0].x.toFixed(2)} ${list[0].y.toFixed(2)}`;
    for (let index = 1; index < list.length - 1; index += 1) {
      const next = list[index + 1];
      const midX = (list[index].x + next.x) / 2;
      const midY = (list[index].y + next.y) / 2;
      path += ` Q ${list[index].x.toFixed(2)} ${list[index].y.toFixed(2)} ${midX.toFixed(2)} ${midY.toFixed(2)}`;
    }
    const last = list[list.length - 1];
    path += ` T ${last.x.toFixed(2)} ${last.y.toFixed(2)}`;
    return path;
  }

  function pointsUntil(points, endPoint) {
    const out = [];
    for (const point of points) {
      out.push(point);
      if (point === endPoint) break;
    }
    return out;
  }

  function pathLengthFromPoints(points) {
    let length = 0;
    for (let index = 0; index < points.length - 1; index += 1) {
      length += distance(points[index], points[index + 1]);
    }
    return length;
  }

  function branchWidthAt(spec, t, width = spec.widthBase) {
    if (spec.isRoot) {
      const carried = 1 - smoothstep(0.68, 1, t);
      return width * (0.58 + carried * 0.42);
    }

    const swell = Math.pow(Math.sin(Math.PI * clamp(t, 0, 1)), 0.82);
    const floor = spec.fine ? 0.11 : 0.16;
    return width * (floor + swell * (1 - floor));
  }

  function groupChildrenByParent(branches) {
    const childrenByParent = new Map();
    for (const branch of branches) {
      if (!branch.parentId) continue;
      if (!childrenByParent.has(branch.parentId)) childrenByParent.set(branch.parentId, []);
      childrenByParent.get(branch.parentId).push(branch);
    }
    return childrenByParent;
  }

  function leafGrowth(branch, progressById) {
    return smoothstep(0.58, 1, progressById.get(branch.id) || 0);
  }

  function resolveLeafMass(branch, progressById, childrenByParent, cache) {
    if (cache.has(branch.id)) return cache.get(branch.id);

    const own = branch.ownLeafMass * leafGrowth(branch, progressById);
    const children = childrenByParent.get(branch.id) || [];
    const total = own + children.reduce(
      (sum, child) => sum + resolveLeafMass(child, progressById, childrenByParent, cache),
      0,
    );
    cache.set(branch.id, total);
    return total;
  }

  function resolveSegmentWidth(branch, position, progressById, childrenByParent, cache) {
    const localProgress = progressById.get(branch.id) || 0;
    const children = childrenByParent.get(branch.id) || [];
    const supportedLeafMass = branch.ownLeafMass * leafGrowth(branch, progressById) + children.reduce((sum, child) => {
      if ((child.attachAt || 0) < position) return sum;
      return sum + resolveLeafMass(child, progressById, childrenByParent, cache);
    }, 0);

    const supportScale = branch.isRoot ? 0.48 : 0.3;
    const supportWidth = branch.widthBase + supportedLeafMass * supportScale;
    const rootFlare = branch.isRoot ? supportedLeafMass * 0.14 : 0;
    const carryingMinimum = branch.isRoot ? 4.8 + supportedLeafMass * 0.08 : 0;
    const structuralWidth = Math.max(supportWidth + rootFlare, carryingMinimum);
    const liveWidth = branchWidthAt(branch, position, structuralWidth);
    const emergence = 0.72 + smoothstep(0.12, 0.6, localProgress) * 0.28;
    return liveWidth * emergence;
  }

  function updateSupportLayers(branch, progressById, childrenByParent, cache) {
    if (!branch.supportLayers) return;

    for (const layer of branch.supportLayers) {
      const growth = smoothstep(0.08, 1, progressById.get(layer.progressId) || 0);
      const width = layer.finalWidth * (0.08 + growth * 0.92);
      layer.node.style.strokeDashoffset = String(layer.length * (1 - growth));
      layer.node.setAttribute("stroke-width", width.toFixed(2));
      layer.node.setAttribute("opacity", (0.04 + growth * 0.64).toFixed(3));
    }
  }

  function distance(a, b) {
    return Math.hypot(b.x - a.x, b.y - a.y);
  }

  function normalize(vector) {
    const magnitude = Math.hypot(vector.x, vector.y) || 1;
    return {
      x: vector.x / magnitude,
      y: vector.y / magnitude,
    };
  }

  function clamp(value, min, max) {
    return Math.min(max, Math.max(min, value));
  }

  function smoothstep(min, max, value) {
    const x = clamp((value - min) / (max - min), 0, 1);
    return x * x * (3 - 2 * x);
  }

  function mulberry32(seed) {
    let value = seed >>> 0;
    return () => {
      value += 0x6d2b79f5;
      let result = Math.imul(value ^ (value >>> 15), 1 | value);
      result ^= result + Math.imul(result ^ (result >>> 7), 61 | result);
      return ((result ^ (result >>> 14)) >>> 0) / 4294967296;
    };
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => init());
  } else {
    init();
  }

  document.addEventListener("htmx:load", (event) => init(event.target));
  window.ASSproutLogo = { init };
})();
