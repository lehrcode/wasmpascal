program QuicksortRecords;
type
  TPerson = record
    Name: string[12];
    Alter: Integer
  end;
  TListe = array[1..8] of TPerson;
var
  Liste: TListe;
  Anzahl, i: Integer;

procedure Neu(const Name: string; Alter: Integer);
begin
  Inc(Anzahl);
  Liste[Anzahl].Name := Name;
  Liste[Anzahl].Alter := Alter
end;

procedure Sortiere(var a: TListe; n: Integer);

  procedure Quick(links, rechts: Integer);
  var
    i, j: Integer;
    Pivot: string;
    h: TPerson;
  begin
    i := links;
    j := rechts;
    Pivot := a[(links + rechts) div 2].Name;
    repeat
      while a[i].Name < Pivot do Inc(i);
      while Pivot < a[j].Name do Dec(j);
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

begin
  Anzahl := 0;
  Neu('Niklaus', 90); Neu('Ada', 36); Neu('Grace', 85); Neu('Edsger', 72);
  Neu('Alan', 41); Neu('Barbara', 50); Neu('Donald', 87); Neu('Anders', 63);
  Sortiere(Liste, Anzahl);
  for i := 1 to Anzahl do
    with Liste[i] do
      WriteLn(Name:12, Alter:4)
end.
