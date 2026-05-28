import {
  areSelectionsEqual,
  type CodeViewDiffItem,
  type CodeViewItem,
  type CodeViewLineSelection,
  type CodeViewOptions,
  type DiffIndicators,
  type DiffLineAnnotation,
  type LineAnnotation,
  type SelectedLineRange,
} from '@pierre/diffs';
import {
  CodeView,
  type CodeViewHandle,
  useStableCallback,
} from '@pierre/diffs/react';
import { IconCheck, IconChevronSm, IconCopy } from '@pierre/icons';
import {
  memo,
  type RefObject,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';

import type { AvatarName } from './annotation-shared';
import { CODE_VIEW_CUSTOM_CSS, CODE_VIEW_LAYOUT } from './constants';
import { DraftAnnotation } from './DraftAnnotation';
import { ExampleAnnotation } from './ExampleAnnotation';
import type {
  CodeViewDeletedCommentEvent,
  CodeViewSavedCommentEvent,
  CommentMetadata,
} from './types';
import {
  classifyCommentLineType,
  isDiffItem,
  isDraftAnnotation,
  isDraftMetadata,
  isSavedAnnotation,
} from './utils';
import { cn } from '@/lib/utils';

function getNextItemVersion(item: CodeViewItem<CommentMetadata>): number {
  return typeof item.version === 'number' ? item.version + 1 : 1;
}

function updateViewerDiffItem(
  viewer: CodeViewHandle<CommentMetadata>,
  itemId: string,
  updateItem: (item: CodeViewDiffItem<CommentMetadata>) => boolean
): CodeViewDiffItem<CommentMetadata> | undefined {
  const item = viewer.getItem(itemId);
  if (item == null || !isDiffItem(item)) {
    return undefined;
  }

  if (!updateItem(item)) {
    return undefined;
  }

  item.version = getNextItemVersion(item);
  return viewer.updateItem(item) ? item : undefined;
}

interface ActiveDraftComment {
  itemId: string;
  key: string;
}

interface CodeViewWrapperProps {
  className?: string;
  diffStyle: 'split' | 'unified';
  onCommentDeleted(comment: CodeViewDeletedCommentEvent): void;
  onCommentSaved(comment: CodeViewSavedCommentEvent): void;
  overflow: 'wrap' | 'scroll';
  showBackgrounds: boolean;
  diffIndicators: DiffIndicators;
  lineNumbers: boolean;
  scrollRef: RefObject<HTMLDivElement | null>;
  viewerRef: RefObject<CodeViewHandle<CommentMetadata> | null>;
  initialItems: CodeViewItem<CommentMetadata>[];
  onLineLinkChange(selection: CodeViewLineSelection | null): void;
  onViewerReady(): void;
  searchQuery: string;
}

const DIFF_SEARCH_HIGHLIGHT_NAME = 'dehub-diff-search';

interface CSSHighlightsRegistry {
  delete(name: string): boolean;
  set(name: string, highlight: unknown): void;
}

interface CSSWithHighlights {
  highlights?: CSSHighlightsRegistry;
}

interface WindowWithHighlight extends Window {
  Highlight?: new (...ranges: Range[]) => unknown;
}

export const CodeViewWrapper = memo(function CodeViewWrapper({
  className,
  diffStyle,
  onCommentDeleted,
  onCommentSaved,
  overflow,
  showBackgrounds,
  diffIndicators,
  lineNumbers,
  scrollRef,
  viewerRef,
  initialItems,
  onLineLinkChange,
  onViewerReady,
  searchQuery,
}: CodeViewWrapperProps) {
  const nextCommentKeyRef = useRef(0);
  const activeDraftRef = useRef<ActiveDraftComment | null>(null);
  const [selectedLines, setSelectedLines] =
    useState<CodeViewLineSelection | null>(null);

  useDiffSearchHighlights(scrollRef, searchQuery);

  const handleSetSelection = useStableCallback(
    (selection: CodeViewLineSelection | null) => {
      setSelectedLines(selection);
    }
  );

  const handleToggleCommentSelection = useStableCallback(
    (selection: CodeViewLineSelection) => {
      setSelectedLines((prev) =>
        prev?.id === selection.id &&
        areSelectionsEqual(prev.range, selection.range)
          ? null
          : selection
      );
    }
  );

  const handleLineSelectionEnd = useStableCallback(
    (range: SelectedLineRange | null, item: CodeViewItem<CommentMetadata>) => {
      if (range == null || item.type !== 'diff') {
        onLineLinkChange(null);
      } else {
        onLineLinkChange({ id: item.id, range });
      }
    }
  );

  const handleViewerRef = useStableCallback(
    (viewer: CodeViewHandle<CommentMetadata> | null) => {
      viewerRef.current = viewer;
      if (viewer != null) {
        onViewerReady();
      }
    }
  );

  const handleCreateDraftComment = useStableCallback(
    (range: SelectedLineRange, itemId: string) => {
      const side = range.endSide ?? range.side;
      if (side == null) {
        return;
      }

      const lineNumber = range.end;
      const commentKey = `draft-${nextCommentKeyRef.current++}`;
      const { current: viewer } = viewerRef;
      if (viewer == null) {
        return;
      }

      const draftAnnotation: DiffLineAnnotation<CommentMetadata> = {
        side,
        lineNumber,
        metadata: {
          kind: 'draft',
          key: commentKey,
          message: '',
          range,
        },
      };

      const { current: activeDraft } = activeDraftRef;
      if (activeDraft != null && activeDraft.itemId !== itemId) {
        updateViewerDiffItem(viewer, activeDraft.itemId, (item) => {
          if (item.annotations == null) {
            return false;
          }

          const nextAnnotations = item.annotations.filter(
            (annotation) => annotation.metadata.key !== activeDraft.key
          );
          if (nextAnnotations.length === item.annotations.length) {
            return false;
          }

          item.annotations = nextAnnotations;
          return true;
        });
      }

      const updatedItem = updateViewerDiffItem(viewer, itemId, (item) => {
        const nonDraftAnnotations = (item.annotations ?? []).filter(
          (annotation) => !isDraftMetadata(annotation.metadata)
        );
        item.annotations = [...nonDraftAnnotations, draftAnnotation];
        return true;
      });

      if (updatedItem != null) {
        activeDraftRef.current = { itemId, key: commentKey };
      }
    }
  );

  const handleRemoveComment = useStableCallback(
    (itemId: string, key: string) => {
      const { current: viewer } = viewerRef;
      if (viewer == null) {
        return;
      }
      const item = viewer.getItem(itemId);
      const removedAnnotation =
        item != null && isDiffItem(item)
          ? item.annotations?.find(
              (annotation) => annotation.metadata.key === key
            )
          : undefined;

      updateViewerDiffItem(viewer, itemId, (item) => {
        if (item.annotations == null) {
          return false;
        }

        const nextAnnotations = item.annotations.filter(
          (annotation) => annotation.metadata.key !== key
        );

        if (nextAnnotations.length === item.annotations.length) {
          return false;
        }

        item.annotations = nextAnnotations;
        return true;
      });

      const { current: activeDraft } = activeDraftRef;
      if (activeDraft?.itemId === itemId && activeDraft.key === key) {
        activeDraftRef.current = null;
      }

      setSelectedLines(null);
      onLineLinkChange(null);
      if (removedAnnotation != null && isSavedAnnotation(removedAnnotation)) {
        onCommentDeleted({ itemId, key });
      }
    }
  );

  const handleSaveDraftComment = useStableCallback(
    (itemId: string, key: string, message: string, author: AvatarName) => {
      const trimmedMessage = message.trim();
      const { current: viewer } = viewerRef;
      if (trimmedMessage.length === 0 || viewer == null) {
        return;
      }

      const item = viewer.getItem(itemId);
      if (item == null || !isDiffItem(item)) {
        return;
      }

      const draftAnnotation = item?.annotations?.find(
        (annotation) => annotation.metadata.key === key
      );
      if (draftAnnotation == null || !isDraftAnnotation(draftAnnotation)) {
        return;
      }

      const updatedItem = updateViewerDiffItem(viewer, itemId, (item) => {
        if (item.annotations == null) {
          return false;
        }

        const nextAnnotations: DiffLineAnnotation<CommentMetadata>[] =
          item.annotations.map((annotation) => {
            if (
              annotation.metadata.key !== key ||
              !isDraftAnnotation(annotation)
            ) {
              return annotation;
            }

            return {
              ...annotation,
              metadata: {
                kind: 'saved',
                key,
                author,
                message: trimmedMessage,
                range: annotation.metadata.range,
              },
            };
          });

        let didChange = false;
        for (let index = 0; index < nextAnnotations.length; index++) {
          if (nextAnnotations[index] !== item.annotations[index]) {
            didChange = true;
            break;
          }
        }

        if (!didChange) {
          return false;
        }

        item.annotations = nextAnnotations;
        return true;
      });

      if (updatedItem == null) {
        return;
      }

      const { current: activeDraft } = activeDraftRef;
      if (activeDraft?.itemId === itemId && activeDraft.key === key) {
        activeDraftRef.current = null;
      }

      setSelectedLines(null);
      onLineLinkChange(null);
      onCommentSaved({
        author,
        itemId,
        key,
        lineNumber: draftAnnotation.lineNumber,
        lineType: classifyCommentLineType(
          item.fileDiff,
          draftAnnotation.side,
          draftAnnotation.lineNumber
        ),
        message: trimmedMessage,
        range: draftAnnotation.metadata.range,
        side: draftAnnotation.side,
      });
    }
  );

  const handleToggleItemCollapsed = useStableCallback((itemId: string) => {
    const { current: viewerHandle } = viewerRef;
    const viewer = viewerHandle?.getInstance();
    const item = viewerHandle?.getItem(itemId);
    if (viewerHandle == null || viewer == null || item == null) {
      return;
    }

    // NOTE(amadeus): If the top of the item is before the scrollTop, then
    // we'll want to apply a scroll fix on the next render to ensure we
    // keep the collapsed file in view and anchored.
    const itemTop = viewer.getTopForItem(itemId);
    item.collapsed = item.collapsed !== true;
    item.version = getNextItemVersion(item);
    if (!viewerHandle.updateItem(item)) {
      return;
    }

    if (itemTop != null && itemTop < viewer.getScrollTop()) {
      viewer.scrollTo({
        type: 'item',
        id: item.id,
        align: 'start',
      });
    }
  });

  const renderCommentAnnotation = useStableCallback(
    (
      annotation:
        | DiffLineAnnotation<CommentMetadata>
        | LineAnnotation<CommentMetadata>,
      item: CodeViewItem<CommentMetadata>
    ) => {
      if (!('side' in annotation) || item.type !== 'diff') {
        return null;
      }

      if (isDraftAnnotation(annotation)) {
        return (
          <DraftAnnotation
            annotation={annotation}
            itemId={item.id}
            onCancel={handleRemoveComment}
            onSave={handleSaveDraftComment}
          />
        );
      }

      if (!isSavedAnnotation(annotation)) {
        return null;
      }

      return (
        <ExampleAnnotation
          annotation={annotation}
          itemId={item.id}
          onDelete={handleRemoveComment}
          onToggleSelection={handleToggleCommentSelection}
        />
      );
    }
  );

  const renderCustomHeader = useStableCallback(
    (item: CodeViewItem<CommentMetadata>) => {
      if (item.type !== 'diff') {
        return null;
      }

      return (
        <CustomDiffHeader
          item={item}
          onToggleCollapsed={() => handleToggleItemCollapsed(item.id)}
        />
      );
    }
  );

  // NOTE(amadeus): For some insane reason, the react compiler did not know how
  // to properly memoize this, so we pulled it into a `useMemo` for safety...
  const options: CodeViewOptions<CommentMetadata> = useMemo(
    () =>
      ({
        // Use this to validate itemMetrics when changing layout with unsafeCSS.
        // __devOnlyValidateItemHeights: true,
        layout: CODE_VIEW_LAYOUT,
        itemMetrics: {
          hunkLineCount: 1,
          lineHeight: 18,
          diffHeaderHeight: 42,
          spacing: 8,
        },
        theme: { dark: 'pierre-dark-soft', light: 'pierre-light-soft' },
        diffStyle,
        diffIndicators,
        overflow,
        disableBackground: !showBackgrounds,
        disableLineNumbers: !lineNumbers,
        lineHoverHighlight: 'number',
        // hunkSeparators: 'line-info-basic',
        enableLineSelection: true,
        enableGutterUtility: false,
        stickyHeaders: true,
        unsafeCSS: CODE_VIEW_CUSTOM_CSS,
        // FIXME(amadeus): Move all `onX` methods onto the react component maybe?
        onLineSelectionEnd(range, context) {
          handleLineSelectionEnd(range, context.item);
        },
      }) satisfies CodeViewOptions<CommentMetadata>,
    [
      diffIndicators,
      diffStyle,
      handleLineSelectionEnd,
      lineNumbers,
      overflow,
      showBackgrounds,
    ]
  );
  return (
    <CodeView<CommentMetadata>
      ref={handleViewerRef}
      containerRef={scrollRef}
      initialItems={initialItems}
      className={cn(
        className,
        'cv-scrollbar relative h-full min-h-0 min-w-0 flex-1 overflow-y-auto overflow-x-clip overscroll-contain border-b border-border w-full [contain:strict] [overflow-anchor:none] md:border-b-0 [&_diffs-container]:overflow-clip [&_diffs-container]:[contain:layout_paint_style] [&_diffs-container]:shadow-[0_-1px_0_var(--color-border-opaque),0_1px_0_var(--color-border-opaque)]'
      )}
      options={options}
      selectedLines={selectedLines}
      onSelectedLinesChange={handleSetSelection}
      renderCustomHeader={renderCustomHeader}
    />
  );
});

function useDiffSearchHighlights(
  scrollRef: RefObject<HTMLDivElement | null>,
  query: string
) {
  useEffect(() => {
    const root = scrollRef.current;
    const normalizedQuery = query.trim().toLowerCase();
    const cssHighlights = (CSS as CSSWithHighlights).highlights;
    const HighlightCtor = (window as WindowWithHighlight).Highlight;

    if (root == null || cssHighlights == null || HighlightCtor == null) {
      return;
    }

    let frame = 0;
    const observedRoots = new WeakSet<Node>();
    const clearHighlight = () => {
      cssHighlights.delete(DIFF_SEARCH_HIGHLIGHT_NAME);
    };
    const observeRoot = (node: Node) => {
      if (observedRoots.has(node)) {
        return;
      }
      observedRoots.add(node);
      observer.observe(node, {
        childList: true,
        characterData: true,
        subtree: true,
      });
    };
    const observeShadowRoots = (node: Node) => {
      if (node instanceof Element && node.shadowRoot != null) {
        observeRoot(node.shadowRoot);
      }
      for (const child of node.childNodes) {
        observeShadowRoots(child);
      }
      if (node instanceof Element && node.shadowRoot != null) {
        for (const child of node.shadowRoot.childNodes) {
          observeShadowRoots(child);
        }
      }
    };
    const forEachTextNode = (node: Node, visit: (textNode: Text) => void) => {
      if (node.nodeType === Node.TEXT_NODE) {
        visit(node as Text);
        return;
      }
      if (node instanceof Element && node.shadowRoot != null) {
        forEachTextNode(node.shadowRoot, visit);
      }
      for (const child of node.childNodes) {
        forEachTextNode(child, visit);
      }
    };
    const updateHighlight = () => {
      frame = 0;
      clearHighlight();
      if (normalizedQuery.length === 0 || !root.isConnected) {
        return;
      }

      const ranges: Range[] = [];
      observeShadowRoots(root);
      forEachTextNode(root, (textNode) => {
        const text = textNode.data;
        const searchableText = text.toLowerCase();
        let start = searchableText.indexOf(normalizedQuery);
        while (start !== -1) {
          const range = root.ownerDocument.createRange();
          range.setStart(textNode, start);
          range.setEnd(textNode, start + normalizedQuery.length);
          ranges.push(range);
          start = searchableText.indexOf(
            normalizedQuery,
            start + normalizedQuery.length
          );
        }
      });

      if (ranges.length > 0) {
        cssHighlights.set(
          DIFF_SEARCH_HIGHLIGHT_NAME,
          new HighlightCtor(...ranges)
        );
      }
    };
    const scheduleUpdate = () => {
      if (frame !== 0) {
        return;
      }
      frame = window.requestAnimationFrame(updateHighlight);
    };
    const observer = new MutationObserver((mutations) => {
      for (const mutation of mutations) {
        for (const node of mutation.addedNodes) {
          observeShadowRoots(node);
        }
      }
      scheduleUpdate();
    });

    scheduleUpdate();
    observeRoot(root);
    observeShadowRoots(root);

    return () => {
      observer.disconnect();
      if (frame !== 0) {
        window.cancelAnimationFrame(frame);
      }
      clearHighlight();
    };
  }, [query, scrollRef]);
}

interface CustomDiffHeaderProps {
  item: CodeViewDiffItem<CommentMetadata>;
  onToggleCollapsed(): void;
}

function CustomDiffHeader({ item, onToggleCollapsed }: CustomDiffHeaderProps) {
  const { additions, deletions } = getDiffLineStats(item);
  const disabled =
    item.fileDiff.splitLineCount === 0 && item.fileDiff.unifiedLineCount === 0;

  return (
    <div className="flex min-w-0 flex-1 items-center justify-between gap-3 px-4 py-2">
      <div className="flex min-w-0 items-center gap-2">
        <CollapseDiffButton
          disabled={disabled}
          collapsed={item.collapsed}
          onToggle={onToggleCollapsed}
        />
        <ChangeTypeIcon type={item.fileDiff.type} />
        <div className="min-w-0 truncate" title={item.fileDiff.name}>
          <bdi>{item.fileDiff.name}</bdi>
        </div>
        <CopyPathButton path={item.fileDiff.name} />
      </div>
      <div className="ml-auto flex shrink-0 items-center gap-2 font-mono">
        {(deletions > 0 || additions === 0) && (
          <span className="text-[var(--diffs-deletion-base)]">-{deletions}</span>
        )}
        {(additions > 0 || deletions === 0) && (
          <span className="text-[var(--diffs-addition-base)]">+{additions}</span>
        )}
      </div>
    </div>
  );
}

function getDiffLineStats(item: CodeViewDiffItem<CommentMetadata>) {
  let additions = 0;
  let deletions = 0;
  for (const hunk of item.fileDiff.hunks) {
    additions += hunk.additionLines;
    deletions += hunk.deletionLines;
  }
  return { additions, deletions };
}

function ChangeTypeIcon({ type }: { type: CodeViewDiffItem<CommentMetadata>['fileDiff']['type'] }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'inline-flex size-3.5 shrink-0 items-center justify-center rounded-[4px] border-2',
        type === 'new' && 'border-[var(--diffs-addition-base)]',
        type === 'deleted' && 'border-[var(--diffs-deletion-base)]',
        type !== 'new' &&
          type !== 'deleted' &&
          'border-[var(--diffs-modified-base)]'
      )}
    />
  );
}

function CopyPathButton({ path }: { path: string }) {
  const [copied, setCopied] = useState(false);
  const timeoutRef = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      if (timeoutRef.current != null) {
        window.clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  return (
    <button
      type="button"
      aria-label={`Copy path ${path}`}
      title={copied ? 'Copied' : 'Copy file path'}
      className={cn(
        'inline-flex size-6 cursor-pointer items-center justify-center rounded-md transition',
        copied
          ? 'bg-emerald-500/15 text-emerald-500 hover:bg-emerald-500/20 hover:text-emerald-400'
          : 'text-muted-foreground hover:bg-muted hover:text-foreground'
      )}
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
        void navigator.clipboard?.writeText(path).then(() => {
          setCopied(true);
          if (timeoutRef.current != null) {
            window.clearTimeout(timeoutRef.current);
          }
          timeoutRef.current = window.setTimeout(() => {
            setCopied(false);
            timeoutRef.current = null;
          }, 1_200);
        });
      }}
    >
      {copied ? (
        <IconCheck aria-hidden="true" className="size-3.5" />
      ) : (
        <IconCopy aria-hidden="true" className="size-3.5" />
      )}
    </button>
  );
}

interface CollapseDiffButtonProps {
  disabled?: boolean;
  collapsed?: boolean;
  onToggle(): void;
}

function CollapseDiffButton({
  disabled = false,
  collapsed = false,
  onToggle,
}: CollapseDiffButtonProps) {
  return (
    <button
      type="button"
      disabled={disabled}
      aria-expanded={!disabled && !collapsed}
      aria-hidden={disabled}
      aria-label={
        disabled ? undefined : collapsed ? 'Expand diff' : 'Collapse diff'
      }
      className="text-muted-foreground hover:bg-muted hover:text-foreground ml-[-8px] inline-flex size-6 cursor-pointer items-center justify-center rounded-md transition disabled:pointer-events-none disabled:opacity-50"
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
        onToggle();
      }}
    >
      <IconChevronSm
        aria-hidden="true"
        className={cn(
          'size-4 transition-transform',
          (disabled || collapsed) && '-rotate-90'
        )}
      />
    </button>
  );
}
