program Quicksort;
type
  TPerson = record
    Name: string[20];
    Alter: Integer
  end;
  TListe = array[1..100] of TPerson;
var
  Liste: TListe;
  Anzahl: Integer;

procedure Sortiere(var a: TListe; n: Integer);

  procedure Quick(links, rechts: Integer);
  var
    i, j: Integer;
    Pivot: Integer;
    h: TPerson;
  begin
    i := links;
    j := rechts;
    Pivot := a[(links + rechts) div 2].Alter;
    repeat
      while a[i].Alter < Pivot do Inc(i);
      while Pivot < a[j].Alter do Dec(j);
      if i <= j then
      begin
        h := a[i]; a[i] := a[j]; a[j] := h;
        Inc(i); Dec(j)
      end
    until i > j;
    if links < j then Quick(links, j);
    if i < rechts then Quick(i, rechts)
  end;

begin
  if n > 1 then Quick(1, n)
end;

procedure Neu(const Name: string; Alter: Integer);
begin
  Inc(Anzahl);
  Liste[Anzahl].Name := Name;
  Liste[Anzahl].Alter := Alter
end;

var
  i: Integer;
begin
  Anzahl := 0;
  Neu('Anna', 34); Neu('Bernd', 21); Neu('Clara', 56); Neu('Dieter', 18);
  Sortiere(Liste, Anzahl);
  for i := 1 to Anzahl do
    with Liste[i] do
      WriteLn(Name:20, Alter:4)
end.
