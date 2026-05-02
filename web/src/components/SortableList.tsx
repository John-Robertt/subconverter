import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent
} from "@dnd-kit/core";
import { SortableContext, arrayMove, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { GripVertical } from "lucide-react";
import type { ReactNode } from "react";

interface SortableListProps<T> {
  items: T[];
  getId: (item: T, index: number) => string;
  onReorder: (items: T[]) => void;
  disabled?: boolean;
  renderItem: (item: T, index: number, handle: ReactNode) => ReactNode;
}

export function SortableList<T>({ items, getId, onReorder, disabled, renderItem }: SortableListProps<T>) {
  const ids = items.map(getId);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates })
  );

  function handleDragEnd(event: DragEndEvent) {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const oldIndex = ids.indexOf(String(active.id));
    const newIndex = ids.indexOf(String(over.id));
    if (oldIndex < 0 || newIndex < 0) return;
    onReorder(arrayMove(items, oldIndex, newIndex));
  }

  if (disabled) {
    return (
      <div className="sortable-list">
        {items.map((item, index) => (
          <div key={getId(item, index)}>{renderItem(item, index, <span className="drag-handle disabled" />)}</div>
        ))}
      </div>
    );
  }

  return (
    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
      <SortableContext items={ids} strategy={verticalListSortingStrategy}>
        <div className="sortable-list">
          {items.map((item, index) => (
            <SortableRow key={getId(item, index)} id={getId(item, index)}>
              {(handle) => renderItem(item, index, handle)}
            </SortableRow>
          ))}
        </div>
      </SortableContext>
    </DndContext>
  );
}

function SortableRow({ id, children }: { id: string; children: (handle: ReactNode) => ReactNode }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition
  };

  const handle = (
    <button className="drag-handle" type="button" aria-label="拖拽排序" {...attributes} {...listeners}>
      <GripVertical size={16} aria-hidden="true" />
    </button>
  );

  return (
    <div ref={setNodeRef} style={style} className={isDragging ? "sortable-row dragging" : "sortable-row"}>
      {children(handle)}
    </div>
  );
}
