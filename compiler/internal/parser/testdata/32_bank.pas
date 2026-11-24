program Bank;
type
  TBuchung = record
    Betrag: Real;
    Text: string[40]
  end;
  TKonto = record
    Nummer: LongInt;
    Inhaber: record
      Vorname, Nachname: string[20]
    end;
    Saldo: Real;
    Anzahl: Integer;
    Buchungen: array[1..50] of TBuchung
  end;
var
  Konten: array[1..10] of TKonto;
  Zins: Real;

procedure Buche(var k: TKonto; Betrag: Real; const Text: string);
begin
  with k do
  begin
    Inc(Anzahl);
    Buchungen[Anzahl].Betrag := Betrag;
    Buchungen[Anzahl].Text := Text;
    Saldo := Saldo + Betrag
  end
end;

function Zinsen(const k: TKonto; Satz: Real; Jahre: Integer): Real;
var
  i: Integer;
  b: Real;
begin
  b := k.Saldo;
  for i := 1 to Jahre do
    b := b * (1 + Satz / 100);
  Zinsen := b - k.Saldo
end;

begin
  Konten[1].Nummer := 4711;
  Konten[1].Inhaber.Vorname := 'Grace';
  Konten[1].Saldo := 0;
  Konten[1].Anzahl := 0;
  Buche(Konten[1], 1.5E3, 'Gehalt');
  Buche(Konten[1], -250.75, 'Miete');
  Zins := Zinsen(Konten[1], 2.5, 10);
  WriteLn(Konten[1].Inhaber.Vorname, ': ', Konten[1].Saldo:10:2, ' +', Zins:8:2)
end.
