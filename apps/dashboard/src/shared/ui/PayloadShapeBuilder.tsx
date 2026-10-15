import { IconPlus, IconTrash } from '@tabler/icons-react';
import { CustomSelect } from './CustomSelect';

export type PayloadFieldType = 'string' | 'number' | 'integer' | 'boolean' | 'date' | 'json' | 'list';

export type PayloadField = {
  id: string;
  name: string;
  type: PayloadFieldType;
  required: boolean;
  description: string;
};

type PayloadShapeBuilderProps = {
  fields: PayloadField[];
  onChange: (fields: PayloadField[]) => void;
};

export const payloadFieldNamePattern = /^[a-zA-Z_][a-zA-Z0-9_]{0,63}$/;

const fieldTypeOptions = [
  { value: 'string', label: 'Text' },
  { value: 'number', label: 'Number' },
  { value: 'integer', label: 'Whole number' },
  { value: 'boolean', label: 'Yes / no' },
  { value: 'date', label: 'Date and time' },
  { value: 'json', label: 'Object' },
  { value: 'list', label: 'List' },
] as const;

function makeFieldId() {
  return `field-${Math.random().toString(36).slice(2, 8)}-${Date.now()}`;
}

export function createPayloadField(required = true): PayloadField {
  return { id: makeFieldId(), name: '', type: 'string', required, description: '' };
}

export function schemaFromPayloadFields(fields: PayloadField[]) {
  const properties: Record<string, Record<string, unknown>> = {};
  const required: string[] = [];
  for (const field of fields) {
    const name = field.name.trim();
    if (!name) continue;
    const propertyType = field.type === 'date' ? 'string' : field.type === 'json' ? 'object' : field.type === 'list' ? 'array' : field.type;
    properties[name] = {
      type: propertyType,
      ...(field.type === 'date' ? { format: 'date-time' } : {}),
      ...(field.description.trim() ? { description: field.description.trim() } : {}),
    };
    if (field.required) required.push(name);
  }
  return { type: 'object', properties, required, additionalProperties: false };
}

export function payloadFieldsFromSchema(schema: Record<string, unknown>): PayloadField[] {
  const properties = schema.properties && typeof schema.properties === 'object' && !Array.isArray(schema.properties) ? schema.properties as Record<string, unknown> : {};
  const required = new Set(Array.isArray(schema.required) ? schema.required.filter((name): name is string => typeof name === 'string') : []);
  const fields = Object.entries(properties).map(([name, definition]) => {
    const field = definition && typeof definition === 'object' && !Array.isArray(definition) ? definition as Record<string, unknown> : {};
    const type = field.type === 'string' && field.format === 'date-time' ? 'date' : field.type === 'object' ? 'json' : field.type === 'array' ? 'list' : field.type;
    return {
      id: makeFieldId(),
      name,
      type: (type === 'number' || type === 'integer' || type === 'boolean' || type === 'date' || type === 'json' || type === 'list' ? type : 'string') as PayloadFieldType,
      required: required.has(name),
      description: typeof field.description === 'string' ? field.description : '',
    };
  });
  return fields.length > 0 ? fields : [createPayloadField(true)];
}

function sampleValue(type: PayloadFieldType) {
  if (type === 'number') return '12.50';
  if (type === 'integer') return '3';
  if (type === 'boolean') return 'Yes';
  if (type === 'date') return '2026-10-14';
  if (type === 'json') return '{ … }';
  if (type === 'list') return '[ 3 items ]';
  return 'Text value';
}

export function PayloadShapeBuilder({ fields, onChange }: PayloadShapeBuilderProps) {
  const namedFields = fields.filter((field) => field.name.trim());
  const requiredCount = namedFields.filter((field) => field.required).length;

  function updateField(id: string, patch: Partial<PayloadField>) {
    onChange(fields.map((field) => field.id === id ? { ...field, ...patch } : field));
  }

  function addField() {
    onChange([...fields, createPayloadField(false)]);
  }

  function removeField(id: string) {
    onChange(fields.length > 1 ? fields.filter((field) => field.id !== id) : [createPayloadField(true)]);
  }

  return <div className="payload-builder">
    <p className="field-help">Add the pieces of information your app will send. For example, an <code>order.completed</code> event might include an order ID, total, and payment status.</p>

    <div className="payload-table">
      <div className="payload-head" aria-hidden="true"><span>Field name</span><span>Value type</span><span>Required</span><span /></div>
      {fields.map((field, index) => {
        const invalidName = Boolean(field.name && !payloadFieldNamePattern.test(field.name.trim()));
        return <div className="payload-row" key={field.id}>
          <div className="payload-row-grid">
            <input className="mono" aria-label={`Field ${index + 1} name`} value={field.name} onChange={(event) => updateField(field.id, { name: event.target.value })} placeholder={index === 0 ? 'order_id' : 'customer_id'} aria-invalid={invalidName} spellCheck={false} />
            <CustomSelect ariaLabel={`Field ${index + 1} value type`} value={field.type} onChange={(value) => updateField(field.id, { type: value as PayloadFieldType })} options={[...fieldTypeOptions]} />
            <label className="payload-required"><input type="checkbox" checked={field.required} onChange={(event) => updateField(field.id, { required: event.target.checked })} aria-label={`Field ${index + 1} is required`} /><span>Required</span></label>
            {fields.length > 1 ? <button className="icon-button ghost danger compact" type="button" onClick={() => removeField(field.id)} aria-label={`Remove field ${index + 1}`}><IconTrash size={15} stroke={1.75} /></button> : <span aria-hidden="true" />}
          </div>
          <input className="payload-description" aria-label={`Field ${index + 1} description`} value={field.description} onChange={(event) => updateField(field.id, { description: event.target.value })} placeholder={field.name ? `Describe ${field.name} for your team (optional)` : 'Describe this field for your team (optional)'} />
          {invalidName && <small className="field-error">Use letters, numbers, and underscores. Start with a letter.</small>}
        </div>;
      })}
    </div>

    <div className="payload-footer">
      <button className="secondary-button compact" type="button" onClick={addField}><IconPlus size={14} stroke={1.75} />Add field</button>
      <span>{namedFields.length ? `${namedFields.length} field${namedFields.length === 1 ? '' : 's'} · ${requiredCount} required` : 'Add at least one field to continue'}</span>
    </div>
  </div>;
}

export function payloadSampleRows(fields: PayloadField[]) {
  return fields.map((field) => ({
    id: field.id,
    name: field.name.trim() || 'field_name',
    type: field.type,
    required: field.required,
    value: sampleValue(field.type),
  }));
}
