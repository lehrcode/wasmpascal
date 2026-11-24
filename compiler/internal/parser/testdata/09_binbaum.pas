program Binaerbaum;
type
  PBaum = ^TBaum;
  TBaum = record
    Schluessel: string;
    Anzahl: Integer;
    Links, Rechts: PBaum
  end;
var
  Wurzel: PBaum;
  Wort: string;

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
    WriteLn(b^.Schluessel, ': ', b^.Anzahl);
    InOrder(b^.Rechts)
  end
end;

function Hoehe(b: PBaum): Integer;
var
  l, r: Integer;
begin
  if b = nil then Hoehe := 0
  else
  begin
    l := Hoehe(b^.Links);
    r := Hoehe(b^.Rechts);
    if l > r then Hoehe := l + 1 else Hoehe := r + 1
  end
end;

begin
  Wurzel := nil;
  repeat
    ReadLn(Wort);
    if Wort <> '' then Einfuegen(Wurzel, Wort)
  until Wort = '';
  InOrder(Wurzel);
  WriteLn('Hoehe: ', Hoehe(Wurzel))
end.
