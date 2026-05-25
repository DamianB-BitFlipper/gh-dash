import { type CodeViewItem, type FileDiffMetadata } from '@pierre/diffs';

export interface CodeViewDiffStats {
  addedLines: number;
  deletedLines: number;
  fileCount: number;
  totalLinesOfCode: number;
}

export interface CodeViewFileEntry {
  addedLines: number;
  deletedLines: number;
  id: string;
  path: string;
  type: FileDiffMetadata['type'];
}

export interface CodeViewDataAccumulator {
  diffStats: CodeViewDiffStats;
  files: CodeViewFileEntry[];
  fileIndex: number;
  items: CodeViewItem[];
  pendingItems: CodeViewItem[];
}

export function createCodeViewDataAccumulator(): CodeViewDataAccumulator {
  return {
    diffStats: {
      addedLines: 0,
      deletedLines: 0,
      fileCount: 0,
      totalLinesOfCode: 0,
    },
    files: [],
    fileIndex: 0,
    items: [],
    pendingItems: [],
  };
}

export function appendFileDiffToCodeViewData(
  accumulator: CodeViewDataAccumulator,
  fileDiff: FileDiffMetadata,
  idPrefix?: string
): void {
  const { diffStats } = accumulator;
  diffStats.fileCount++;
  diffStats.totalLinesOfCode += fileDiff.unifiedLineCount;
  for (const hunk of fileDiff.hunks) {
    diffStats.addedLines += hunk.additionLines;
    diffStats.deletedLines += hunk.deletionLines;
  }

  const path = fileDiff.name || `file-${accumulator.fileIndex + 1}`;
  const id = idPrefix == null ? path : `${idPrefix}/${path}`;
  const addedLines = fileDiff.hunks.reduce(
    (count, hunk) => count + hunk.additionLines,
    0
  );
  const deletedLines = fileDiff.hunks.reduce(
    (count, hunk) => count + hunk.deletionLines,
    0
  );
  const item: CodeViewItem = {
    id,
    type: 'diff',
    fileDiff,
    version: 0,
  };

  accumulator.fileIndex++;
  accumulator.files.push({
    addedLines,
    deletedLines,
    id,
    path,
    type: fileDiff.type,
  });
  accumulator.items.push(item);
  accumulator.pendingItems.push(item);
}

export function takePendingCodeViewItems(
  accumulator: CodeViewDataAccumulator
): CodeViewItem[] {
  const { pendingItems } = accumulator;
  accumulator.pendingItems = [];
  return pendingItems;
}
