program ConstParameter;
type
  TPunkt = record x, y: Integer end;

procedure Verschiebe(const p: TPunkt);
begin
  p.x := p.x + 1
end;

begin
end.
