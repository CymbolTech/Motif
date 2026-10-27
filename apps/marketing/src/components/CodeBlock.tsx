// Code that wraps on narrow screens without losing its nesting: each source
// line is its own block, and a wrapped line continues two columns in from
// where the line itself starts.

export function CodeBlock({ code, className = '' }: { code: string; className?: string }) {
  return (
    <pre className={`code-block ${className}`}>
      <code>
        {code.split('\n').map((line, index) => {
          const indent = line.length - line.trimStart().length;
          return (
            <span key={index} className="code-line" style={{ paddingLeft: `${indent + 2}ch`, textIndent: '-2ch' }}>
              {line.trimStart() || ' '}
            </span>
          );
        })}
      </code>
    </pre>
  );
}
