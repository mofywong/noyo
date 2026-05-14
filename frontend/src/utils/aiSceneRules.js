export const SCENE_COORD_SCALE = 10000;
export const DETECTOR_FRAME_SIZE = 640;

const clamp = (value, min, max) => Math.min(max, Math.max(min, value));

export const clampRoiPoint = (point = {}) => ({
  x: clamp(Number(point.x ?? point.X ?? 0), 0, SCENE_COORD_SCALE),
  y: clamp(Number(point.y ?? point.Y ?? 0), 0, SCENE_COORD_SCALE),
});

export const normalizeCanvasPoint = (point, canvasSize) => ({
  x: clamp((point.x / canvasSize.width) * SCENE_COORD_SCALE, 0, SCENE_COORD_SCALE),
  y: clamp((point.y / canvasSize.height) * SCENE_COORD_SCALE, 0, SCENE_COORD_SCALE),
});

export const denormalizeRoiPoint = (point, canvasSize) => {
  const p = clampRoiPoint(point);
  return {
    x: (p.x / SCENE_COORD_SCALE) * canvasSize.width,
    y: (p.y / SCENE_COORD_SCALE) * canvasSize.height,
  };
};

export const nearestPointIndex = (points, target, canvasSize, radius = 12) => {
  if (!Array.isArray(points)) return -1;
  let nearest = -1;
  let nearestDistance = radius;
  points.forEach((point, index) => {
    const displayPoint = denormalizeRoiPoint(point, canvasSize);
    const distance = Math.hypot(displayPoint.x - target.x, displayPoint.y - target.y);
    if (distance <= nearestDistance) {
      nearest = index;
      nearestDistance = distance;
    }
  });
  return nearest;
};

export const resolveVideoRenderBox = (videoSize, containerSize, fitMode = 'contain') => {
  const videoWidth = Number(videoSize?.width || 0);
  const videoHeight = Number(videoSize?.height || 0);
  const containerWidth = Number(containerSize?.width || 0);
  const containerHeight = Number(containerSize?.height || 0);
  if (videoWidth <= 0 || videoHeight <= 0 || containerWidth <= 0 || containerHeight <= 0) {
    return {
      width: containerWidth,
      height: containerHeight,
      offsetX: 0,
      offsetY: 0,
    };
  }

  const videoRatio = videoWidth / videoHeight;
  const containerRatio = containerWidth / containerHeight;
  const cover = fitMode === 'cover';
  const matchWidth = cover ? videoRatio < containerRatio : videoRatio > containerRatio;

  if (matchWidth) {
    const width = containerWidth;
    const height = containerWidth / videoRatio;
    return {
      width,
      height,
      offsetX: 0,
      offsetY: (containerHeight - height) / 2,
    };
  }

  const height = containerHeight;
  const width = containerHeight * videoRatio;
  return {
    width,
    height,
    offsetX: (containerWidth - width) / 2,
    offsetY: 0,
  };
};

export const videoPointToDetectorPoint = (point, videoSize) => {
  const videoWidth = Number(videoSize?.width || 0);
  const videoHeight = Number(videoSize?.height || 0);
  if (videoWidth <= 0 || videoHeight <= 0) {
    return {
      x: clamp(Number(point?.x || 0), 0, DETECTOR_FRAME_SIZE),
      y: clamp(Number(point?.y || 0), 0, DETECTOR_FRAME_SIZE),
    };
  }

  const scale = Math.min(DETECTOR_FRAME_SIZE / videoWidth, DETECTOR_FRAME_SIZE / videoHeight);
  const padX = (DETECTOR_FRAME_SIZE - videoWidth * scale) / 2;
  const padY = (DETECTOR_FRAME_SIZE - videoHeight * scale) / 2;
  return {
    x: clamp(Number(point?.x || 0) * scale + padX, 0, DETECTOR_FRAME_SIZE),
    y: clamp(Number(point?.y || 0) * scale + padY, 0, DETECTOR_FRAME_SIZE),
  };
};

export const detectorPointToVideoPoint = (point, videoSize) => {
  const videoWidth = Number(videoSize?.width || 0);
  const videoHeight = Number(videoSize?.height || 0);
  if (videoWidth <= 0 || videoHeight <= 0) {
    return {
      x: clamp(Number(point?.x || 0), 0, DETECTOR_FRAME_SIZE),
      y: clamp(Number(point?.y || 0), 0, DETECTOR_FRAME_SIZE),
    };
  }

  const scale = Math.min(DETECTOR_FRAME_SIZE / videoWidth, DETECTOR_FRAME_SIZE / videoHeight);
  const padX = (DETECTOR_FRAME_SIZE - videoWidth * scale) / 2;
  const padY = (DETECTOR_FRAME_SIZE - videoHeight * scale) / 2;
  return {
    x: clamp((Number(point?.x || 0) - padX) / scale, 0, videoWidth),
    y: clamp((Number(point?.y || 0) - padY) / scale, 0, videoHeight),
  };
};

export const detectorPointToRoiPoint = (point) => ({
  x: clamp((Number(point?.x || 0) / DETECTOR_FRAME_SIZE) * SCENE_COORD_SCALE, 0, SCENE_COORD_SCALE),
  y: clamp((Number(point?.y || 0) / DETECTOR_FRAME_SIZE) * SCENE_COORD_SCALE, 0, SCENE_COORD_SCALE),
});

export const roiPointToDetectorPoint = (point) => {
  const p = clampRoiPoint(point);
  return {
    x: (p.x / SCENE_COORD_SCALE) * DETECTOR_FRAME_SIZE,
    y: (p.y / SCENE_COORD_SCALE) * DETECTOR_FRAME_SIZE,
  };
};

export const displayPointToRoiPoint = (point, canvasSize, videoSize, fitMode = 'contain') => {
  if (!videoSize?.width || !videoSize?.height) {
    return normalizeCanvasPoint(point, canvasSize);
  }

  const renderBox = resolveVideoRenderBox(videoSize, canvasSize, fitMode);
  const videoPoint = {
    x: clamp((point.x - renderBox.offsetX) / renderBox.width, 0, 1) * videoSize.width,
    y: clamp((point.y - renderBox.offsetY) / renderBox.height, 0, 1) * videoSize.height,
  };
  return detectorPointToRoiPoint(videoPointToDetectorPoint(videoPoint, videoSize));
};

export const roiPointToDisplayPoint = (point, canvasSize, videoSize, fitMode = 'contain') => {
  if (!videoSize?.width || !videoSize?.height) {
    return denormalizeRoiPoint(point, canvasSize);
  }

  const renderBox = resolveVideoRenderBox(videoSize, canvasSize, fitMode);
  const detectorPoint = roiPointToDetectorPoint(point);
  const videoPoint = detectorPointToVideoPoint(detectorPoint, videoSize);
  return {
    x: renderBox.offsetX + (videoPoint.x / videoSize.width) * renderBox.width,
    y: renderBox.offsetY + (videoPoint.y / videoSize.height) * renderBox.height,
  };
};
