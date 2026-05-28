'use client';

import {
  type CodeViewItem,
  type DiffIndicators,
  type SelectionSide,
} from '@pierre/diffs';
import { type CodeViewHandle, useWorkerPool } from '@pierre/diffs/react';
import {
  type ReactNode,
  type Ref,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';

import { preloadAvatars } from './annotation-shared';
import { CodeViewSidebar } from './CodeViewSidebar';
import { CodeViewStatusPanel } from './CodeViewStatusPanel';
import { CodeViewWrapper } from './CodeViewWrapper';
import type {
  CodeViewDeletedCommentEvent,
  CodeViewSavedCommentEntry,
  CodeViewSavedCommentEvent,
  CommentMetadata,
} from './types';
import { usePatchLoader } from './usePatchLoader';
import {
  removeSavedCommentSidebarEntry,
  upsertSavedCommentSidebarEntry,
} from './utils';
import { cn } from '@/lib/utils';

interface ReviewUIProps {
  domain?: string;
  path: string;
}

export function ReviewUI({ domain, path }: ReviewUIProps) {
  useEffect(preloadAvatars, []);

  const isWorkerPoolReadyOrDisable = useIsWorkerPoolReadyOrDisabled();
  const [diffStyle, setDiffStyle] = useState<'split' | 'unified'>('unified');
  const [collapseMode, setCollapseMode] = useState<'expanded' | 'collapsed'>(
    'expanded'
  );
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [fileTreeOverlayOpen, setFileTreeOverlayOpen] = useState(false);
  const [overflow, setOverflow] = useState<'wrap' | 'scroll'>('wrap');
  const [showBackgrounds, setShowBackgrounds] = useState(true);
  const [diffIndicators, setDiffIndicators] = useState<DiffIndicators>('bars');
  const [lineNumbers, setLineNumbers] = useState(true);
  const scrollRef = useRef<HTMLDivElement>(null);
  const viewerRef = useRef<CodeViewHandle<CommentMetadata> | null>(null);
  const handlePatchLoadStart = useCallback(() => {
    setFileTreeOverlayOpen(false);
  }, []);
  const {
    applyCollapseModeToLoaded,
    commentFileByItemId,
    commentSections,
    diffStats,
    errorMessage,
    getSearchItems,
    initialItems,
    loadState,
    onLineLinkChange,
    onViewerReady,
    retryLoad,
    searchItemsVersion,
    setCommentSections,
    treeSource,
    viewerKey,
  } = usePatchLoader({
    collapseMode,
    domain,
    onLoadStart: handlePatchLoadStart,
    path,
    viewerRef,
  });

  useEffect(() => {
    const mediaQuery = window.matchMedia('(max-width: 767px)');
    const updateMobileState = (matches: boolean) => {
      setDiffStyle(matches ? 'unified' : 'split');
      if (!matches) setFileTreeOverlayOpen(false);
    };
    const handleChange = (event: MediaQueryListEvent) => {
      updateMobileState(event.matches);
    };

    updateMobileState(mediaQuery.matches);
    mediaQuery.addEventListener('change', handleChange);
    return () => mediaQuery.removeEventListener('change', handleChange);
  }, []);
  const handleSelectTreeItem = useCallback((itemId: string) => {
    setFileTreeOverlayOpen(false);
    const viewer = viewerRef.current;
    if (viewer == null) {
      return;
    }
    const item = viewer.getItem(itemId);
    if (item != null && item.collapsed === true) {
      item.collapsed = false;
      item.version = typeof item.version === 'number' ? item.version + 1 : 1;
      viewer.updateItem(item);
    }
    viewer.scrollTo({
      type: 'item',
      id: itemId,
      align: 'start',
      behavior: 'smooth',
    });
  }, []);
  const handleToggleCollapseMode = useCallback(() => {
    const next = collapseMode === 'expanded' ? 'collapsed' : 'expanded';
    setCollapseMode(next);
    applyCollapseModeToLoaded(next);
  }, [applyCollapseModeToLoaded, collapseMode]);
  const handleCommentSaved = useCallback(
    (comment: CodeViewSavedCommentEvent) => {
      setCommentSections((prev) =>
        upsertSavedCommentSidebarEntry(prev, commentFileByItemId, comment)
      );
    },
    [commentFileByItemId, setCommentSections]
  );
  const handleCommentDeleted = useCallback(
    (comment: CodeViewDeletedCommentEvent) => {
      setCommentSections((prev) =>
        removeSavedCommentSidebarEntry(prev, comment)
      );
    },
    [setCommentSections]
  );
  const handleToggleFileTreeOverlay = useCallback(() => {
    setFileTreeOverlayOpen((open) => !open);
  }, []);
  const handleToggleSidebarCollapsed = useCallback(() => {
    setSidebarCollapsed((collapsed) => !collapsed);
  }, []);
  const handleCloseFileTreeOverlay = useCallback(() => {
    setFileTreeOverlayOpen(false);
  }, []);
  const handleSelectComment = useCallback(
    (comment: CodeViewSavedCommentEntry) => {
      setFileTreeOverlayOpen(false);
      viewerRef.current?.setSelectedLines({
        id: comment.itemId,
        range: comment.range,
      });
      viewerRef.current?.scrollTo({
        type: 'line',
        id: comment.itemId,
        lineNumber: comment.range.end,
        side: comment.range.endSide ?? comment.range.side,
        align: 'center',
        behavior: 'smooth-auto',
      });
    },
    []
  );
  const searchInputRef = useRef<HTMLInputElement>(null);
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchIndex, setSearchIndex] = useState(0);
  const searchMatches = useMemo(
    () => buildDiffSearchMatches(getSearchItems(), searchQuery),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [searchItemsVersion, searchQuery]
  );
  const currentSearchMatch = searchMatches[searchIndex] ?? null;

  const focusSearchInput = useCallback(() => {
    window.requestAnimationFrame(() => {
      searchInputRef.current?.focus();
      searchInputRef.current?.select();
    });
  }, []);
  const openSearch = useCallback(() => {
    setSearchOpen(true);
    focusSearchInput();
  }, [focusSearchInput]);
  const closeSearch = useCallback(() => {
    setSearchOpen(false);
    viewerRef.current?.clearSelectedLines();
  }, []);
  const moveSearch = useCallback(
    (direction: 1 | -1) => {
      if (searchMatches.length === 0) {
        return;
      }
      setSearchIndex((index) =>
        (index + direction + searchMatches.length) % searchMatches.length
      );
    },
    [searchMatches.length]
  );

  useEffect(() => {
    setSearchIndex(0);
  }, [searchQuery, searchMatches.length]);

  useEffect(() => {
    if (!searchOpen || currentSearchMatch == null) {
      return;
    }
    scrollToDiffSearchMatch(viewerRef.current, currentSearchMatch);
  }, [currentSearchMatch, searchOpen]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'f') {
        event.preventDefault();
        openSearch();
        return;
      }

      if (!searchOpen) {
        return;
      }

      if (event.key === 'Escape') {
        event.preventDefault();
        closeSearch();
        return;
      }

      if (event.key === 'Enter') {
        event.preventDefault();
        moveSearch(event.shiftKey ? -1 : 1);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [closeSearch, moveSearch, openSearch, searchOpen]);
  const viewerAvailable =
    isWorkerPoolReadyOrDisable &&
    (loadState === 'ready' ||
      (loadState === 'streaming' && initialItems.length > 0));

  return (
    <ReviewGrid sidebarCollapsed={sidebarCollapsed}>
      {viewerAvailable && treeSource != null ? (
        <>
          <CodeViewSidebar
            className="[grid-area:viewer] md:[grid-area:tree]"
            collapseMode={collapseMode}
            commentSections={commentSections}
            diffIndicators={diffIndicators}
            diffStyle={diffStyle}
            diffStats={diffStats}
            lineNumbers={lineNumbers}
            mobileOverlayOpen={fileTreeOverlayOpen}
            onMobileClose={handleCloseFileTreeOverlay}
            onToggleCollapseMode={handleToggleCollapseMode}
            onToggleSidebarCollapsed={handleToggleSidebarCollapsed}
            overflow={overflow}
            scrollRef={scrollRef}
            setDiffIndicators={setDiffIndicators}
            setDiffStyle={setDiffStyle}
            setLineNumbers={setLineNumbers}
            setOverflow={setOverflow}
            setShowBackgrounds={setShowBackgrounds}
            showBackgrounds={showBackgrounds}
            sidebarCollapsed={sidebarCollapsed}
            source={treeSource}
            streaming={loadState === 'streaming'}
            onSelectItem={handleSelectTreeItem}
          />
          <CodeViewWrapper
            key={viewerKey}
            className="[grid-area:viewer]"
            diffStyle={diffStyle}
            overflow={overflow}
            showBackgrounds={showBackgrounds}
            diffIndicators={diffIndicators}
            lineNumbers={lineNumbers}
            scrollRef={scrollRef}
            viewerRef={viewerRef}
            initialItems={initialItems}
            onCommentDeleted={handleCommentDeleted}
            onCommentSaved={handleCommentSaved}
            onLineLinkChange={onLineLinkChange}
            onViewerReady={onViewerReady}
            searchQuery={searchOpen ? searchQuery : ''}
          />
          {searchOpen && (
            <DiffSearchOverlay
              inputRef={searchInputRef}
              matchCount={searchMatches.length}
              query={searchQuery}
              selectedIndex={searchIndex}
              streaming={loadState === 'streaming'}
              onChange={setSearchQuery}
              onClose={closeSearch}
              onNext={() => moveSearch(1)}
              onPrevious={() => moveSearch(-1)}
            />
          )}
        </>
      ) : (
        <CodeViewStatusPanel
          state={loadState}
          errorMessage={errorMessage}
          onRetry={retryLoad}
        />
      )}
    </ReviewGrid>
  );
}

interface DiffSearchMatch {
  itemId: string;
  lineNumber?: number;
  side?: SelectionSide;
}

function buildDiffSearchMatches(
  items: readonly CodeViewItem<CommentMetadata>[],
  query: string
): DiffSearchMatch[] {
  const normalizedQuery = query.trim().toLowerCase();
  if (normalizedQuery.length === 0) {
    return [];
  }

  const matches: DiffSearchMatch[] = [];
  for (const item of items) {
    if (item.type !== 'diff') {
      continue;
    }

    if (item.fileDiff.name.toLowerCase().includes(normalizedQuery)) {
      matches.push({ itemId: item.id });
    }

    forEachRenderedDiffLine(item, ({ lineNumber, side, text }) => {
      if (text.toLowerCase().includes(normalizedQuery)) {
        matches.push({ itemId: item.id, lineNumber, side });
      }
    });
  }
  return matches;
}

function forEachRenderedDiffLine(
  item: CodeViewItem<CommentMetadata>,
  visit: (line: { lineNumber: number; side: SelectionSide; text: string }) => void
) {
  if (item.type !== 'diff') {
    return;
  }

  const { fileDiff } = item;
  for (const hunk of fileDiff.hunks) {
    let additionLineIndex = hunk.additionLineIndex;
    let deletionLineIndex = hunk.deletionLineIndex;
    let additionLineNumber = hunk.additionStart;
    let deletionLineNumber = hunk.deletionStart;

    for (const content of hunk.hunkContent) {
      if (content.type === 'context') {
        for (let index = 0; index < content.lines; index++) {
          visit({
            lineNumber: additionLineNumber + index,
            side: 'additions',
            text: fileDiff.additionLines[additionLineIndex + index] ?? '',
          });
        }
        additionLineIndex += content.lines;
        deletionLineIndex += content.lines;
        additionLineNumber += content.lines;
        deletionLineNumber += content.lines;
        continue;
      }

      for (let index = 0; index < content.deletions; index++) {
        visit({
          lineNumber: deletionLineNumber + index,
          side: 'deletions',
          text: fileDiff.deletionLines[deletionLineIndex + index] ?? '',
        });
      }
      for (let index = 0; index < content.additions; index++) {
        visit({
          lineNumber: additionLineNumber + index,
          side: 'additions',
          text: fileDiff.additionLines[additionLineIndex + index] ?? '',
        });
      }
      deletionLineIndex += content.deletions;
      additionLineIndex += content.additions;
      deletionLineNumber += content.deletions;
      additionLineNumber += content.additions;
    }
  }
}

function scrollToDiffSearchMatch(
  viewer: CodeViewHandle<CommentMetadata> | null,
  match: DiffSearchMatch
) {
  if (viewer == null) {
    return;
  }

  const item = viewer.getItem(match.itemId);
  if (item == null) {
    return;
  }

  if (item.collapsed === true) {
    item.collapsed = false;
    item.version = typeof item.version === 'number' ? item.version + 1 : 1;
    viewer.updateItem(item);
    viewer.getInstance()?.render(true);
  }

  if (match.lineNumber == null) {
    viewer.clearSelectedLines();
    viewer.scrollTo({
      type: 'item',
      id: match.itemId,
      align: 'start',
      behavior: 'smooth-auto',
    });
    return;
  }

  const range = {
    start: match.lineNumber,
    end: match.lineNumber,
    side: match.side,
    endSide: match.side,
  };
  viewer.setSelectedLines({ id: match.itemId, range });
  viewer.scrollTo({
    type: 'line',
    id: match.itemId,
    lineNumber: match.lineNumber,
    side: match.side,
    align: 'center',
    behavior: 'smooth-auto',
  });
}

interface DiffSearchOverlayProps {
  matchCount: number;
  query: string;
  selectedIndex: number;
  streaming: boolean;
  onChange(query: string): void;
  onClose(): void;
  onNext(): void;
  onPrevious(): void;
  inputRef: Ref<HTMLInputElement>;
}

function DiffSearchOverlay({
  inputRef,
  matchCount,
  query,
  selectedIndex,
  streaming,
  onChange,
  onClose,
  onNext,
  onPrevious,
}: DiffSearchOverlayProps) {
  const status =
    query.trim().length === 0
      ? 'Find in diff'
      : matchCount === 0
        ? 'No results'
        : `${selectedIndex + 1} / ${matchCount}`;

  return (
    <div className="fixed top-3 right-3 z-50 flex max-w-[calc(100vw-24px)] items-center gap-1 rounded-lg border border-border bg-background/95 p-1.5 text-sm shadow-xl backdrop-blur">
      <input
        ref={inputRef}
        aria-label="Find in diff"
        className="h-8 w-56 min-w-0 rounded-md border border-border bg-background px-2 outline-none focus:border-ring focus:ring-2 focus:ring-ring/35"
        placeholder="Find in diff"
        value={query}
        onChange={(event) => onChange(event.target.value)}
      />
      <span className="min-w-20 px-2 text-center text-xs text-muted-foreground">
        {status}
        {streaming && query.trim().length > 0 ? '...' : ''}
      </span>
      <button
        type="button"
        className="hover:bg-muted inline-flex h-8 w-12 items-center justify-center rounded-md text-muted-foreground hover:text-foreground disabled:opacity-40"
        disabled={matchCount === 0}
        aria-label="Previous match"
        onClick={onPrevious}
      >
        Prev
      </button>
      <button
        type="button"
        className="hover:bg-muted inline-flex h-8 w-12 items-center justify-center rounded-md text-muted-foreground hover:text-foreground disabled:opacity-40"
        disabled={matchCount === 0}
        aria-label="Next match"
        onClick={onNext}
      >
        Next
      </button>
      <button
        type="button"
        className="hover:bg-muted inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:text-foreground"
        aria-label="Close search"
        onClick={onClose}
      >
        x
      </button>
    </div>
  );
}

function useIsWorkerPoolReadyOrDisabled() {
  const workerPool = useWorkerPool();
  const [isReady, setIsReady] = useState(
    () => workerPool?.isInitialized() ?? true
  );
  const isReadyRef = useRef(isReady);
  useEffect(() => {
    // The callback will always be fired immediately with the new state, so we
    // don't need to check for it in the effect
    return workerPool?.subscribeToStatChanges((stats) => {
      const isReady = stats.managerState === 'initialized';
      if (isReady !== isReadyRef.current) {
        setIsReady(isReady);
        isReadyRef.current = isReady;
      }
    });
  }, [workerPool]);
  return isReady;
}

interface ReviewGridProps {
  children: ReactNode;
  sidebarCollapsed: boolean;
}

function ReviewGrid({ children, sidebarCollapsed }: ReviewGridProps) {
  return (
    <div
      className={cn(
        "grid min-h-0 flex-1 grid-cols-1 grid-rows-[minmax(0,1fr)] overflow-hidden overscroll-contain contain-strict [grid-template-areas:'viewer'] md:[grid-template-areas:'tree_viewer']",
        sidebarCollapsed
          ? 'md:grid-cols-[48px_minmax(0,1fr)]'
          : 'md:grid-cols-[320px_minmax(0,1fr)]'
      )}
    >
      {children}
    </div>
  );
}
