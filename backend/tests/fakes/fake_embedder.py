"""FakeEmbedder - Epic 26.7."""

from __future__ import annotations

import hashlib


class FakeEmbedder:
    """根据文本 hash 生成确定性 512 维向量."""

    DIM = 512

    def embed(self, texts: list[str]) -> list[list[float]]:
        out: list[list[float]] = []
        for t in texts:
            h = hashlib.sha256(t.encode("utf-8")).digest()
            vec = [b / 255.0 for b in h] + [0.0] * (self.DIM - len(h))
            out.append(vec[: self.DIM])
        return out
