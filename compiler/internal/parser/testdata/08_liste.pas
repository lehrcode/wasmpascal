program VerketteteListe;
type
  PKnoten = ^TKnoten;
  TKnoten = record
    Wert: Integer;
    Naechster: PKnoten
  end;
var
  Kopf, p, q: PKnoten;
  i: Integer;

procedure Vorne(var Kopf: PKnoten; w: Integer);
var
  neu: PKnoten;
begin
  New(neu);
  neu^.Wert := w;
  neu^.Naechster := Kopf;
  Kopf := neu
end;

function Laenge(p: PKnoten): Integer;
var
  n: Integer;
begin
  n := 0;
  while p <> nil do
  begin
    Inc(n);
    p := p^.Naechster
  end;
  Laenge := n
end;

begin
  Kopf := nil;
  for i := 1 to 5 do Vorne(Kopf, i * i);
  p := Kopf;
  while p <> nil do
  begin
    Write(p^.Wert, ' ');
    p := p^.Naechster
  end;
  WriteLn('(', Laenge(Kopf), ' Elemente)');
  { Liste freigeben }
  p := Kopf;
  while p <> nil do
  begin
    q := p^.Naechster;
    Dispose(p);
    p := q
  end
end.
