program Binaerbaum;
type
  PBaum = ^TBaum;
  TBaum = record
    Schluessel: string[20];
    Anzahl: Integer;
    Links, Rechts: PBaum
  end;
const
  Woerter: array[1..9] of string[10] =
    ('wenn', 'fliegen', 'hinter', 'fliegen', 'fliegen', 'fliegen', 'hinterher', 'wenn', 'die');
var
  Wurzel: PBaum;
  i: Integer;

procedure Einfuegen(var b: PBaum; const s: string);
begin
  if b = nil then
  begin
    New(b);
    b^.Schluessel := s;
    b^.Anzahl := 1;
    b^.Links := nil;
    b^.Rechts := nil
  end
  else if s < b^.Schluessel then
    Einfuegen(b^.Links, s)
  else if s > b^.Schluessel then
    Einfuegen(b^.Rechts, s)
  else
    Inc(b^.Anzahl)
end;

procedure InOrder(b: PBaum);
begin
  if b <> nil then
  begin
    InOrder(b^.Links);
    WriteLn(b^.Schluessel:10, b^.Anzahl:3);
    InOrder(b^.Rechts)
  end
end;

function Hoehe(b: PBaum): Integer;
var
  l, r: Integer;
begin
  if b = nil then
    Hoehe := 0
  else
  begin
    l := Hoehe(b^.Links);
    r := Hoehe(b^.Rechts);
    if l > r then Hoehe := l + 1 else Hoehe := r + 1
  end
end;

procedure Freigeben(b: PBaum);
begin
  if b <> nil then
  begin
    Freigeben(b^.Links);
    Freigeben(b^.Rechts);
    Dispose(b)
  end
end;

begin
  Wurzel := nil;
  for i := 1 to 9 do
    Einfuegen(Wurzel, Woerter[i]);
  InOrder(Wurzel);
  WriteLn('Hoehe: ', Hoehe(Wurzel));
  Freigeben(Wurzel)
end.
